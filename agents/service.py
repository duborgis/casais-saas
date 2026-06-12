import asyncio
import logging
import os
import time
from typing import Any, Optional

from agent import build_agent
from config import AgentConfig, CrewConfig, RuntimeContext
from memory import CoupleMemory

logger = logging.getLogger(__name__)

SESSION_TTL = 3600  # 1 hour


def is_orchestrator(agent_config: Optional[AgentConfig]) -> bool:
    """Only the user-facing orchestrator reads/writes couple memory;
    specialists are stateless delegation targets."""
    role = (
        agent_config.role
        if agent_config and agent_config.role
        else os.environ.get("AGENT_ROLE", "orchestrator")
    )
    return role == "orchestrator"


class AgentService:
    """Service that manages agent instances and their sessions."""
    def __init__(self, memory: Optional[CoupleMemory] = None) -> None:
        """Initialize the agent service with an empty session cache."""
        self._session_agents: dict[str, tuple[Any, float]] = {}
        self._lock = asyncio.Lock()
        self._memory = memory

    def _build_agent(
        self,
        session_id: str,
        agent_config: Optional[AgentConfig],
        crew_config: Optional[CrewConfig],
        runtime_context: RuntimeContext,
        user_id: str,
    ) -> Any:
        """Internal helper to build a new agent instance."""
        return build_agent(agent_config, runtime_context, crew_config, user_id=user_id, session_id=session_id)

    async def get_or_create_agent(
        self,
        session_id: str,
        agent_config: Optional[AgentConfig],
        crew_config: Optional[CrewConfig],
        runtime_context: RuntimeContext,
        user_id: str,
    ) -> Any:
        """Retrieve an existing agent for the session or create a new one."""
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
            # Cold start / new session: rehydrate the conversation from
            # AgentCore Memory so the agent remembers the couple.
            if self._memory and is_orchestrator(agent_config):
                seed = self._memory.load_messages(user_id, session_id)
                if seed:
                    agent.messages = seed + list(agent.messages)
            self._session_agents[session_id] = (agent, now)
            logger.info("Created new session %s", session_id)
            return agent

    async def shutdown(self) -> None:
        """Clear all active sessions."""
        async with self._lock:
            self._session_agents.clear()
