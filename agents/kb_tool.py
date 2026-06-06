"""Bedrock Knowledge Base retrieval tool for casais-saas agents."""
import json
import logging
import os

import boto3
from strands import tool

logger = logging.getLogger(__name__)

_KB_ID = os.environ.get("CASAIS_KB_ID", "")
_REGION = os.environ.get("AWS_REGION", "us-east-1")


def create_kb_tool(kb_id: str = "", region: str = ""):
    """Return a retrieve_knowledge tool bound to the given KB."""
    resolved_kb_id = kb_id or _KB_ID
    resolved_region = region or _REGION

    @tool
    def retrieve_knowledge(query: str, max_results: int = 5) -> str:
        """Search the couples therapy knowledge base for relevant techniques, exercises, and frameworks.

        Use this to find evidence-based approaches before guiding the couple through a dynamic.

        Args:
            query: Natural language query about a technique, exercise, or concept.
            max_results: Number of results to return (1-10).

        Returns:
            Relevant excerpts from the knowledge base.
        """
        if not resolved_kb_id:
            return "[KB not configured — set CASAIS_KB_ID env var]"

        try:
            client = boto3.client("bedrock-agent-runtime", region_name=resolved_region)
            response = client.retrieve(
                knowledgeBaseId=resolved_kb_id,
                retrievalQuery={"text": query},
                retrievalConfiguration={
                    "vectorSearchConfiguration": {"numberOfResults": max(1, min(10, max_results))}
                },
            )
            results = response.get("retrievalResults", [])
            if not results:
                return "Nenhum conteúdo relevante encontrado na base de conhecimento."

            parts = []
            for i, r in enumerate(results, 1):
                text = r.get("content", {}).get("text", "")
                score = r.get("score", 0)
                source = r.get("location", {}).get("s3Location", {}).get("uri", "")
                parts.append(f"[{i}] (score={score:.2f}) {source}\n{text}")

            return "\n\n---\n\n".join(parts)

        except Exception as exc:
            logger.error("KB retrieval failed | query=%s err=%s", query, exc)
            return f"[Erro ao consultar base de conhecimento: {exc}]"

    return retrieve_knowledge
