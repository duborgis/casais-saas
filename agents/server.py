"""
AgentCore-compatible HTTP server for the Strands agent.

Accepts both legacy payload:
  { "sessionId": "...", "input": { "text": "..." } }

And extended payload:
  { "sessionId": "...", "input": { "text": "...", "agentId": "...", "crewId": "...",
    "runtimeContext": {...}, "agentConfig": {...}, "crewConfig": {...} } }
"""

import json
import logging
import os
from contextlib import asynccontextmanager
from contextvars import ContextVar
from typing import Any, Optional

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel, Field, field_validator

from config import AgentConfig, CrewConfig, RuntimeContext, Settings
from service import AgentService

settings = Settings()

_log_format = os.environ.get("LOG_FORMAT", settings.log_format)
logging.basicConfig(
    level=logging.INFO,
    format=(
        '{"time":"%(asctime)s","level":"%(levelname)s","logger":"%(name)s","message":"%(message)s"}'
        if _log_format == "json"
        else "%(asctime)s %(levelname)s %(name)s %(message)s"
    ),
)
logger = logging.getLogger(__name__)

# Propagates trace_id through async tasks within a single request
_trace_id_var: ContextVar[str] = ContextVar("trace_id", default="")


# ── Pydantic models ──────────────────────────────────────────────────────────

class RuntimeContextInput(BaseModel):
    runId: str = ""
    parentRunId: str = ""
    conversationId: str = ""
    delegationDepth: int = 0
    traceId: str = ""


class InvokeInput(BaseModel):
    text: str = Field(min_length=1)
    userId: str = ""
    agentId: str = ""
    crewId: str = ""
    runtimeContext: RuntimeContextInput = Field(default_factory=RuntimeContextInput)
    agentConfig: dict[str, Any] = Field(default_factory=dict)
    crewConfig: dict[str, Any] = Field(default_factory=dict)

    @field_validator("text")
    @classmethod
    def text_not_blank(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("text must not be blank")
        return v


class InvokeRequest(BaseModel):
    sessionId: str = ""
    input: InvokeInput


class InvokeResponse(BaseModel):
    output: dict


# ── Helpers ──────────────────────────────────────────────────────────────────

def _parse_configs(req: InvokeRequest) -> tuple[Optional[AgentConfig], Optional[CrewConfig], RuntimeContext]:
    inp = req.input
    runtime_context = RuntimeContext(
        run_id=inp.runtimeContext.runId,
        parent_run_id=inp.runtimeContext.parentRunId,
        conversation_id=inp.runtimeContext.conversationId,
        delegation_depth=inp.runtimeContext.delegationDepth,
        trace_id=inp.runtimeContext.traceId,
    )
    agent_config: Optional[AgentConfig] = None
    crew_config: Optional[CrewConfig] = None
    if inp.agentConfig:
        agent_config = AgentConfig.model_validate(inp.agentConfig)
    if inp.crewConfig:
        crew_config = CrewConfig.model_validate(inp.crewConfig)
        if crew_config.coordinator_agent_id and not agent_config:
            arn = crew_config.get_member_arn(crew_config.coordinator_agent_id)
            agent_config = AgentConfig(id=crew_config.coordinator_agent_id, runtime_arn=arn)
    return agent_config, crew_config, runtime_context


# ── App ──────────────────────────────────────────────────────────────────────

_svc = AgentService()


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("AgentCore server ready.")
    yield
    await _svc.shutdown()


app = FastAPI(lifespan=lifespan)


@app.exception_handler(Exception)
async def _unhandled_exception(request: Request, exc: Exception) -> JSONResponse:
    logger.exception("Unhandled exception on %s %s", request.method, request.url.path)
    return JSONResponse(status_code=500, content={"detail": "Internal server error"})


@app.get("/health")
async def health() -> dict:
    return {"status": "ok"}


@app.post("/invocations")
async def invocations(request: InvokeRequest) -> InvokeResponse:
    trace_id = request.input.runtimeContext.traceId
    _trace_id_var.set(trace_id)
    agent_config, crew_config, runtime_context = _parse_configs(request)
    try:
        agent = await _svc.get_or_create_agent(
            request.sessionId, agent_config, crew_config, runtime_context, request.input.userId
        )
        user_message = request.input.text
        logger.info("Session=%s traceId=%s | input=%s", request.sessionId, trace_id, user_message[:120])
        result = agent(user_message)
        response_text = str(result)
        logger.info("Session=%s traceId=%s | output=%s", request.sessionId, trace_id, response_text[:120])
        return InvokeResponse(output={"text": response_text})
    except Exception as e:
        logger.exception("Agent invocation failed traceId=%s", trace_id)
        raise HTTPException(status_code=500, detail="Agent invocation failed") from e


@app.post("/invocations/stream")
async def invocations_stream(request: InvokeRequest) -> StreamingResponse:
    trace_id = request.input.runtimeContext.traceId
    _trace_id_var.set(trace_id)
    user_message = request.input.text
    logger.info("Stream | Session=%s traceId=%s | input=%s", request.sessionId, trace_id, user_message[:120])
    agent_config, crew_config, runtime_context = _parse_configs(request)
    agent = await _svc.get_or_create_agent(
        request.sessionId, agent_config, crew_config, runtime_context, request.input.userId
    )

    async def generate():
        current_tool: dict | None = None
        try:
            async for event in agent.stream_async(user_message):
                if text := event.get("data"):
                    yield f"data: {json.dumps({'type': 'chunk', 'data': text})}\n\n"

                elif reasoning := event.get("reasoningText"):
                    yield f"data: {json.dumps({'type': 'thinking', 'data': reasoning})}\n\n"

                elif event.get("type") == "tool_use_stream":
                    tool = event["current_tool_use"]
                    tool_id = tool["toolUseId"]
                    tool_name = tool["name"]
                    tool_input = tool.get("input", "")

                    if not current_tool or current_tool["toolUseId"] != tool_id:
                        if current_tool:
                            prev = current_tool
                            ev_end = {
                                "type": "tool_end",
                                "toolUseId": prev["toolUseId"],
                                "input": prev.get("input", ""),
                                "toolName": prev["toolName"],
                            }
                            yield f"data: {json.dumps(ev_end)}\n\n"
                        if tool_name == "call_agent":
                            ev = {"type": "agent_delegation_start", "toolUseId": tool_id, "toolName": tool_name}
                            yield f"data: {json.dumps(ev)}\n\n"
                        else:
                            ev = {"type": "tool_start", "toolUseId": tool_id, "toolName": tool_name}
                            yield f"data: {json.dumps(ev)}\n\n"

                    current_tool = {"toolUseId": tool_id, "toolName": tool_name, "input": tool_input}

                elif message := event.get("message"):
                    if isinstance(message, dict) and message.get("role") == "user":
                        for block in message.get("content", []):
                            if tr := block.get("toolResult"):
                                result_text = "".join(
                                    c.get("text", "")
                                    for c in tr.get("content", [])
                                    if isinstance(c, dict)
                                )
                                tool_use_id = tr["toolUseId"]
                                is_delegation = current_tool and current_tool.get("toolName") == "call_agent"
                                event_type = "agent_delegation_result" if is_delegation else "tool_result"
                                ev = {"type": event_type, "toolUseId": tool_use_id, "result": result_text[:2000]}
                                yield f"data: {json.dumps(ev)}\n\n"

        except Exception:
            logger.exception("Stream error | Session=%s traceId=%s", request.sessionId, trace_id)
            yield f"data: {json.dumps({'type': 'error', 'message': 'Stream error'})}\n\n"
        finally:
            if current_tool:
                tool_name = current_tool.get("toolName", "")
                end_type = "agent_delegation_end" if tool_name == "call_agent" else "tool_end"
                ev = {
                    "type": end_type,
                    "toolUseId": current_tool["toolUseId"],
                    "input": current_tool.get("input", ""),
                    "toolName": tool_name,
                }
                yield f"data: {json.dumps(ev)}\n\n"
            yield f"data: {json.dumps({'type': 'done'})}\n\n"

    return StreamingResponse(
        generate(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=settings.port)
