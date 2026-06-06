"""Agent factory — routes to the correct role via AGENT_ROLE env var."""
import os
from typing import Any, Optional

from config import AgentConfig, CrewConfig, RuntimeContext

_ROLE = os.environ.get("AGENT_ROLE", "orchestrator")

_ROLE_MAP = {
    "orchestrator": "roles.orchestrator",
    "comunicacao": "roles.comunicacao",
    "conflitos": "roles.conflitos",
    "intimidade": "roles.intimidade",
    "gratidao": "roles.gratidao",
    "objetivos": "roles.objetivos",
}

_BUILDER_MAP = {
    "orchestrator": "build_orchestrator_agent",
    "comunicacao": "build_comunicacao_agent",
    "conflitos": "build_conflitos_agent",
    "intimidade": "build_intimidade_agent",
    "gratidao": "build_gratidao_agent",
    "objetivos": "build_objetivos_agent",
}


def build_agent(
    agent_config: Optional[AgentConfig],
    runtime_context: RuntimeContext,
    crew_config: Optional[CrewConfig] = None,
    user_id: str = "",
    session_id: str = "",
) -> Any:
    role = (agent_config.role if agent_config and agent_config.role else _ROLE) or "orchestrator"

    module_name = _ROLE_MAP.get(role)
    builder_name = _BUILDER_MAP.get(role)

    if not module_name or not builder_name:
        raise ValueError(f"Unknown AGENT_ROLE: '{role}'. Valid roles: {list(_ROLE_MAP)}")

    import importlib
    module = importlib.import_module(module_name)
    builder = getattr(module, builder_name)

    if role == "orchestrator":
        return builder(agent_config, runtime_context, crew_config, user_id, session_id)
    else:
        return builder(agent_config, runtime_context, user_id, session_id)
