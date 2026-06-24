export type ChatRequest = {
  user_id: string;
  session_id: string;
  message: string;
};

export type ChatStreamEvent = {
  type: "agent.started" | "agent.delta" | "agent.completed" | "agent.error" | string;
  user_id?: string;
  session_id?: string;
  intent?: string;
  delta?: string;
  answer?: string;
  error?: string;
  timestamp?: string;
};

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
    const detail = await response.text();
    throw new Error(detail || `请求失败：${response.status}`);
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
