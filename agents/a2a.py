"""A2A delegation tool for casais-saas orchestrator."""
import json
import logging
import os
import time
import uuid
from typing import Optional

import boto3
from strands import tool

from config import AgentConfig, CrewConfig, RuntimeContext

logger = logging.getLogger(__name__)


def create_call_agent_tool(
    agent_config: AgentConfig,
    runtime_context: RuntimeContext,
    crew_config: Optional[CrewConfig] = None,
):
    """Factory that returns a call_agent tool bound to the current agent's context."""
    allowed_ids: set[str] = set()
    arn_registry: dict[str, str] = {}
    use_crew_whitelist = crew_config is not None

    if crew_config:
        for edge in crew_config.delegation_graph:
            if edge.from_agent_id == agent_config.id:
                allowed_ids.add(edge.to_agent_id)
        for member in crew_config.members:
            arn = crew_config.get_member_arn(member.agent_id)
            if arn:
                arn_registry[member.agent_id] = arn
    else:
        allowed_ids = set(agent_config.delegation_policy.allowed_agent_ids)

    max_depth = agent_config.delegation_policy.max_depth
    aws_region = agent_config.region or os.environ.get("AWS_REGION", "us-east-1")

    @tool
    def call_agent(agent_id: str, task: str, context: str = "{}") -> str:
        """Delegar uma dinâmica ou tarefa para um agente especialista do sistema de casais.

        Agentes disponíveis:
        - comunicacao: especialista em comunicação, linguagens do amor, escuta ativa, CNV
        - conflitos: especialista em resolução de conflitos, regulação emocional, método Gottman
        - intimidade: especialista em intimidade emocional, teoria do apego, rituais de conexão
        - gratidao: especialista em gratidão, apreciação, exercícios de admiração mútua
        - objetivos: especialista em planejamento de casal, visão compartilhada, metas

        Args:
            agent_id: ID do agente especialista ('comunicacao', 'conflitos', 'intimidade', 'gratidao', 'objetivos').
            task: Descrição clara da tarefa ou dinâmica a ser conduzida com o casal.
            context: JSON com contexto adicional (histórico da sessão, informações do casal).

        Returns:
            Resposta do agente especialista.
        """
        if runtime_context.delegation_depth >= max_depth:
            return (
                f"[ERRO] Profundidade máxima de delegação ({max_depth}) atingida. "
                f"Não é possível delegar para '{agent_id}'."
            )

        if (use_crew_whitelist or allowed_ids) and agent_id not in allowed_ids:
            return (
                f"[ERRO] Delegação para '{agent_id}' não autorizada. "
                f"Agentes disponíveis: {sorted(allowed_ids)}"
            )

        target_arn = arn_registry.get(agent_id, "")
        if not target_arn:
            return f"[ERRO] ARN não encontrado para o agente '{agent_id}'. Verifique a configuração do crew."

        child_run_id = str(uuid.uuid4())
        payload = {
            "sessionId": child_run_id,
            "input": {
                "text": task,
                "agentId": agent_id,
                "runtimeContext": {
                    "runId": child_run_id,
                    "parentRunId": runtime_context.run_id,
                    "conversationId": runtime_context.conversation_id,
                    "delegationDepth": runtime_context.delegation_depth + 1,
                    "traceId": runtime_context.trace_id,
                },
            },
        }

        logger.info("A2A | from=orchestrator to=%s depth=%d", agent_id, runtime_context.delegation_depth + 1)

        start_ms = int(time.time() * 1000)
        try:
            client = boto3.client("bedrock-agentcore", region_name=aws_region)
            response = client.invoke_agent_runtime(
                agentRuntimeArn=target_arn,
                qualifier="DEFAULT",
                contentType="application/json",
                accept="application/json",
                payload=json.dumps(payload).encode(),
            )
            result = json.loads(response["response"].read())
            duration_ms = int(time.time() * 1000) - start_ms

            text = result.get("output", {}).get("text", str(result))
            if len(text) > 5000:
                text = text[:5000] + "\n... [truncado]"

            logger.info("A2A done | agent=%s duration_ms=%d", agent_id, duration_ms)
            return f"[Agente '{agent_id}' respondeu em {duration_ms}ms]\n\n{text}"

        except Exception as exc:
            duration_ms = int(time.time() * 1000) - start_ms
            logger.error("A2A falhou | agent=%s err=%s", agent_id, exc)
            return f"[ERRO] Chamada ao agente '{agent_id}' falhou após {duration_ms}ms: {exc}"

    return call_agent
