export type ChatRequest = {
  user_id: string;
  session_id: string;
  message: string;
};

export type ConversationMessage = {
  id: string;
  user_id: string;
  session_id: string;
  role: "user" | "assistant" | string;
  content: string;
  status: string;
  created_at: string;
  updated_at: string;
};

export type ChatStreamEvent = {
  type: "agent.started" | "agent.delta" | "agent.completed" | "agent.error" | string;
  trace_id?: string;
  user_id?: string;
  session_id?: string;
  intent?: string;
  delta?: string;
  answer?: string;
  error?: string | { code?: string; message?: string; trace_id?: string };
  error_code?: string;
  timestamp?: string;
};

export type ListMessagesResponse = {
  messages: ConversationMessage[];
  next_before_id?: string;
  has_more: boolean;
};

export type PromptTrace = {
  trace_id: string;
  user_id: string;
  session_id: string;
  intent: string;
  model_task: string;
  used_memory_ids: number[];
  used_history_ids: string[];
  memory_count: number;
  history_message_count: number;
  prompt_chars: number;
  estimated_prompt_tokens: number;
  prompt_builder_version: string;
  system_prompt_hash: string;
  context_snapshot_enabled: boolean;
  prompt?: string;
  created_at: string;
};

export type ReconstructedPrompt = {
  trace_id: string;
  prompt: string;
  prompt_chars: number;
  source: string;
};

export type TokenReport = {
  trace_id: string;
  prompt: string;
  prompt_chars: number;
  estimated_prompt_tokens: number;
  tokenizer: string;
  tokens: Array<{ index: number; text: string; token_id?: number }>;
};

export async function listMessages(userId: string, sessionId: string, turns = 5, beforeId = "") {
  const params = new URLSearchParams({
    user_id: userId,
    session_id: sessionId,
    turns: String(turns),
  });
  if (beforeId) {
    params.set("before_id", beforeId);
  }
  const response = await fetch(`/api/v1/agent/messages?${params.toString()}`);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  const payload = (await response.json()) as Partial<ListMessagesResponse>;
  return {
    messages: payload.messages ?? [],
    next_before_id: payload.next_before_id,
    has_more: payload.has_more ?? false,
  };
}

export async function getPromptTrace(traceId: string) {
  const response = await fetch(`/api/v1/debug/traces/${encodeURIComponent(traceId)}`);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }
  return (await response.json()) as PromptTrace;
}

export async function reconstructPrompt(traceId: string) {
  const response = await fetch(`/api/v1/debug/traces/${encodeURIComponent(traceId)}/reconstructed-prompt`);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }
  return (await response.json()) as ReconstructedPrompt;
}

export async function getTokenReport(traceId: string) {
  const response = await fetch(`/api/v1/debug/traces/${encodeURIComponent(traceId)}/tokens`);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }
  return (await response.json()) as TokenReport;
}

type StreamCallbacks = {
  onEvent: (event: ChatStreamEvent) => void;
  signal?: AbortSignal;
};

export async function streamChat(request: ChatRequest, callbacks: StreamCallbacks) {
  const response = await fetch("/api/v1/agent/chat/stream", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(request),
    signal: callbacks.signal,
  });

  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }
  if (!response.body) {
    throw new Error("浏览器不支持流式响应");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    const blocks = buffer.split("\n\n");
    buffer = blocks.pop() ?? "";

    for (const block of blocks) {
      const event = parseSSEBlock(block);
      if (event) {
        callbacks.onEvent(event);
      }
    }
  }

  buffer += decoder.decode();
  const finalEvent = parseSSEBlock(buffer);
  if (finalEvent) {
    callbacks.onEvent(finalEvent);
  }
}

function parseSSEBlock(block: string): ChatStreamEvent | null {
  const dataLines = block
    .split("\n")
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice(5).trimStart());

  if (dataLines.length === 0) {
    return null;
  }

  try {
    return JSON.parse(dataLines.join("\n")) as ChatStreamEvent;
  } catch {
    return {
      type: "agent.error",
      error: "无法解析服务端事件",
    };
  }
}

async function readErrorMessage(response: Response) {
  const detail = await response.text();
  if (!detail) {
    return `请求失败：${response.status}`;
  }

  try {
    const payload = JSON.parse(detail) as { error?: { code?: string; message?: string; trace_id?: string } };
    if (payload.error?.message) {
      const suffix = payload.error.trace_id ? `（trace_id: ${payload.error.trace_id}）` : "";
      return `${payload.error.code ?? "error"}: ${payload.error.message}${suffix}`;
    }
  } catch {
    return detail;
  }
  return detail;
}
