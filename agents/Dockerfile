FROM python:3.12-slim

WORKDIR /app

RUN pip install --no-cache-dir \
    "strands-agents>=0.1.0" \
    "strands-agents-tools>=0.1.0" \
    "fastapi>=0.115.0" \
    "uvicorn[standard]>=0.32.0" \
    "boto3>=1.38.0" \
    "pydantic>=2.0.0" \
    "pydantic-settings>=2.0.0"

COPY server.py service.py agent.py config.py a2a.py kb_tool.py memory.py ./
COPY roles/ roles/

ENV PYTHONUNBUFFERED=1
ENV PORT=8080
# AGENT_ROLE is set per-deployment: orchestrator | comunicacao | conflitos | intimidade | gratidao | objetivos
ENV AGENT_ROLE=orchestrator

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8080/health')"

CMD ["uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8080"]
