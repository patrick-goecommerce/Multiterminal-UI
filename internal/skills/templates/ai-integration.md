You are integrating AI/LLM capabilities into an application. Follow these principles:

- **API design:** use the official SDKs (Anthropic, OpenAI). Handle rate limits with exponential backoff and retry logic. Set timeouts for all API calls — LLM responses can be slow.
- **Prompt engineering:** be specific and structured. Use system prompts for persona/rules, user prompts for the task. Provide examples (few-shot) for complex output formats. Iterate on prompts like code — version control them.
- **Structured output:** request JSON or structured responses. Use tool/function calling for reliable extraction. Validate LLM output against schemas (Zod, Pydantic) before processing.
- **RAG (Retrieval-Augmented Generation):** chunk documents at semantic boundaries (paragraphs, sections), not fixed token counts. Use embeddings for retrieval (text-embedding-3-small, voyage). Re-rank retrieved chunks before injecting into context.
- **Context management:** track token usage. Truncate or summarize conversation history to fit context windows. Use sliding window or summarization for long conversations. Always leave headroom for the response.
- **Streaming:** use streaming responses for better UX in chat interfaces. Process chunks as they arrive. Handle stream interruptions gracefully with retry logic.
- **Cost control:** cache identical requests. Use smaller models (Haiku, GPT-4o-mini) for classification, routing, and simple tasks. Reserve large models (Opus, GPT-4) for complex reasoning. Log token usage per request for cost attribution.
- **Error handling:** handle API errors (rate limits, overloaded, invalid request) with specific retry strategies. Provide fallback responses when the API is unavailable. Never expose raw API errors to end users.
- **Safety:** implement input validation — reject prompt injection attempts. Use content filtering on outputs. Set appropriate `max_tokens` to prevent runaway costs. Log prompts and responses for audit (respecting privacy).
- **Embeddings:** normalize vectors before storage. Use cosine similarity for retrieval. Store in a vector database (pgvector, Pinecone, Qdrant) with metadata filtering. Batch embedding requests for efficiency.
- **Agents/Tools:** define tools with clear descriptions and typed parameters. Validate tool call arguments before execution. Implement tool result formatting that gives the model useful context. Set maximum iteration limits to prevent loops.
- **Evaluation:** build eval datasets for your use cases. Measure accuracy, latency, and cost. Use LLM-as-judge for subjective quality assessment. Run evals before deploying prompt changes.
- **Caching:** cache embeddings for repeated documents. Cache LLM responses for deterministic queries (temperature=0). Use semantic caching for similar (not identical) queries when appropriate.
