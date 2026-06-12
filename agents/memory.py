"""AgentCore Memory — durable conversation memory for the couple.

Short-term memory via raw events (create_event / list_events). On a cold
start the orchestrator is rehydrated from the current session's events;
if the session is empty (fresh login), it falls back to the actor's most
recent previous session so the couple is remembered across logins.

Memory failures are logged and swallowed: memory must never break chat.
"""
import logging
from datetime import datetime, timezone
from typing import Any

import boto3

logger = logging.getLogger(__name__)

_MAX_EVENTS = 20
_MAX_SESSIONS = 50
_EPOCH = datetime.fromtimestamp(0, tz=timezone.utc)


class CoupleMemory:
    """Thin wrapper over the bedrock-agentcore data plane for one memory store."""

    def __init__(self, memory_id: str, region: str) -> None:
        self.memory_id = memory_id
        self._client = boto3.client("bedrock-agentcore", region_name=region) if memory_id else None

    @property
    def enabled(self) -> bool:
        return bool(self.memory_id)

    def load_messages(self, actor_id: str, session_id: str) -> list[dict[str, Any]]:
        """Return past turns in Strands message format to seed a fresh agent."""
        if not self.enabled or not actor_id or not session_id:
            return []
        try:
            events = self._events(actor_id, session_id)
            if not events:
                prev = self._latest_other_session(actor_id, session_id)
                if prev:
                    events = self._events(actor_id, prev)
            messages: list[dict[str, Any]] = []
            for event in events:
                for item in event.get("payload", []):
                    conv = item.get("conversational") or {}
                    text = (conv.get("content") or {}).get("text", "")
                    if not text:
                        continue
                    role = "user" if conv.get("role") == "USER" else "assistant"
                    messages.append({"role": role, "content": [{"text": text}]})
            if messages:
                logger.info("Memória hidratada: %d mensagens (actor=%s)", len(messages), actor_id)
            return messages
        except Exception:
            logger.exception("Falha ao carregar memória (actor=%s)", actor_id)
            return []

    def save_turn(self, actor_id: str, session_id: str, user_text: str, assistant_text: str) -> None:
        """Persist one user/assistant exchange as a single event."""
        if not self.enabled or not actor_id or not session_id:
            return
        try:
            self._client.create_event(
                memoryId=self.memory_id,
                actorId=actor_id,
                sessionId=session_id,
                eventTimestamp=datetime.now(timezone.utc),
                payload=[
                    {"conversational": {"content": {"text": user_text}, "role": "USER"}},
                    {"conversational": {"content": {"text": assistant_text}, "role": "ASSISTANT"}},
                ],
            )
        except Exception:
            logger.exception("Falha ao gravar memória (actor=%s)", actor_id)

    def _events(self, actor_id: str, session_id: str) -> list[dict[str, Any]]:
        try:
            resp = self._client.list_events(
                memoryId=self.memory_id,
                actorId=actor_id,
                sessionId=session_id,
                includePayloads=True,
                maxResults=_MAX_EVENTS,
            )
        except self._client.exceptions.ResourceNotFoundException:
            return []
        events = resp.get("events", [])
        events.sort(key=lambda e: e.get("eventTimestamp") or _EPOCH)
        return events

    def _latest_other_session(self, actor_id: str, current_session: str) -> str:
        try:
            resp = self._client.list_sessions(
                memoryId=self.memory_id, actorId=actor_id, maxResults=_MAX_SESSIONS
            )
        except self._client.exceptions.ResourceNotFoundException:
            return ""
        sessions = [
            s for s in resp.get("sessionSummaries", []) if s.get("sessionId") != current_session
        ]
        if not sessions:
            return ""
        sessions.sort(key=lambda s: s.get("createdAt") or _EPOCH, reverse=True)
        return sessions[0].get("sessionId", "")
