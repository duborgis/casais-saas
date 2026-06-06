from __future__ import annotations

import re
from typing import Optional

from pydantic import BaseModel, Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

_ARN_RE = re.compile(r"^arn:[a-z0-9\-]+:[a-z0-9\-]+:[a-z0-9\-]*:[0-9]*:.+$")


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    bedrock_model_id: str = "us.amazon.nova-pro-v1:0"
    aws_region: str = "us-east-1"
    port: int = 8080
    agentcore_memory_id: str = ""
    casais_kb_id: str = ""
    agent_role: str = "orchestrator"
    log_format: str = "text"


class DelegationPolicy(BaseModel):
    allowed_agent_ids: list[str] = Field(default_factory=list)
    max_depth: int = 3
    timeout_seconds: int = 60


class RuntimeContext(BaseModel):
    run_id: str = ""
    parent_run_id: str = ""
    conversation_id: str = ""
    delegation_depth: int = 0
    trace_id: str = ""
    couple_id: str = ""


class AgentConfig(BaseModel):
    id: str = ""
    name: str = ""
    description: str = ""
    role: str = ""
    system_prompt: Optional[str] = None
    model_id: str = "us.amazon.nova-pro-v1:0"
    region: str = "us-east-1"
    runtime_arn: str = ""
    tools: list[str] = Field(default_factory=list)
    delegation_policy: DelegationPolicy = Field(default_factory=DelegationPolicy)
    enabled: bool = True

    @field_validator("runtime_arn")
    @classmethod
    def validate_runtime_arn(cls, v: str) -> str:
        if v and not _ARN_RE.match(v):
            raise ValueError("runtime_arn must be a valid AWS ARN")
        return v


class DelegationEdge(BaseModel):
    from_agent_id: str
    to_agent_id: str


class CrewMemberConfig(BaseModel):
    agent_id: str = Field(min_length=1)
    runtime_arn: str = ""
    agent_config: Optional[AgentConfig] = None


class CrewConfig(BaseModel):
    id: str = ""
    name: str = ""
    description: str = ""
    objective: str = ""
    coordinator_agent_id: str = ""
    members: list[CrewMemberConfig] = Field(default_factory=list)
    delegation_graph: list[DelegationEdge] = Field(default_factory=list)
    shared_instructions: str = ""
    enabled: bool = True

    def get_member_arn(self, agent_id: str) -> str:
        for m in self.members:
            if m.agent_id == agent_id:
                if m.runtime_arn:
                    return m.runtime_arn
                if m.agent_config and m.agent_config.runtime_arn:
                    return m.agent_config.runtime_arn
        return ""

    def can_delegate(self, from_id: str, to_id: str) -> bool:
        return any(
            e.from_agent_id == from_id and e.to_agent_id == to_id for e in self.delegation_graph
        )
