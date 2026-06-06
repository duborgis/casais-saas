import asyncio
import logging
import time
from typing import Any, Optional

from agent import build_agent
from config import AgentConfig, CrewConfig, RuntimeContext

logger = logging.getLogger(__name__)

SESSION_TTL = 3600  # 1 hour


class AgentService:
    def __init__(self) -> None:
        self._session_agents: dict[str, tuple[Any, float]] = {}
        self._lock = asyncio.Lock()

    def _build_agent(
        self,
        session_id: str,
        agent_config: Optional[AgentConfig],
        crew_config: Optional[CrewConfig],
        runtime_context: RuntimeContext,
        user_id: str,
    ) -> Any:
        return build_agent(agent_config, runtime_context, crew_config, user_id=user_id, session_id=session_id)

    async def get_or_create_agent(
        self,
        session_id: str,
        agent_config: Optional[AgentConfig],
        crew_config: Optional[CrewConfig],
        runtime_context: RuntimeContext,
        user_id: str,
    ) -> Any:
        if not session_id:
            return self._build_agent("", agent_config, crew_config, runtime_context, user_id)

        async with self._lock:
            now = time.time()
            stale = [k for k, (_, ts) in self._session_agents.items() if now - ts > SESSION_TTL]
            for k in stale:
                logger.info("Evicting stale session %s", k)
                del self._session_agents[k]

            if session_id in self._session_agents:
                agent, _ = self._session_agents[session_id]
                self._session_agents[session_id] = (agent, now)
                logger.info("Reusing session %s (messages=%d)", session_id, len(agent.messages))
                return agent

            agent = self._build_agent(session_id, agent_config, crew_config, runtime_context, user_id)
            self._session_agents[session_id] = (agent, now)
            logger.info("Created new session %s", session_id)
            return agent

    async def shutdown(self) -> None:
        async with self._lock:
            self._session_agents.clear()
