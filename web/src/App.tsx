import {
  Bot,
  BrainCircuit,
  CheckCircle2,
  CircleAlert,
  Loader2,
  PanelLeft,
  RotateCcw,
  Send,
  Sparkles,
  User,
} from "lucide-react";
import { FormEvent, useMemo, useRef, useState } from "react";
import { streamChat, type ChatStreamEvent } from "./lib/api";

type Message = {
  id: string;
  role: "user" | "assistant";
  content: string;
  intent?: string;
  status?: "streaming" | "done" | "error";
};

const examples = [
  "帮我制定一个 6 周 Go 并发学习计划",
  "用题目检查我对 HTTP 缓存的理解",
  "复盘我今天学习数据库索引的内容",
];

function App() {
  const [userId, setUserId] = useState("demo");
  const [sessionId, setSessionId] = useState("default");
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "welcome",
      role: "assistant",
      content: "你好，我是你的 Learning Agent。可以让我制定学习计划、生成练习、回答资料问题或帮你复盘。",
      status: "done",
    },
  ]);
  const [status, setStatus] = useState<"idle" | "connecting" | "streaming" | "done" | "error">("idle");
  const [lastIntent, setLastIntent] = useState("待识别");
  const [error, setError] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  const canSend = input.trim().length > 0 && status !== "connecting" && status !== "streaming";
  const statusText = useMemo(() => {
    switch (status) {
      case "connecting":
        return "连接中";
      case "streaming":
        return "生成中";
      case "done":
        return "已完成";
      case "error":
        return "出错";
      default:
        return "待命";
    }
  }, [status]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const text = input.trim();
    if (!text || !canSend) {
      return;
    }

    const assistantId = crypto.randomUUID();
    setInput("");
    setError("");
    setStatus("connecting");
    setMessages((current) => [
      ...current,
      { id: crypto.randomUUID(), role: "user", content: text, status: "done" },
      { id: assistantId, role: "assistant", content: "", status: "streaming" },
    ]);

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      await streamChat(
        {
          user_id: userId.trim() || "anonymous",
          session_id: sessionId.trim() || "default",
          message: text,
        },
        {
          signal: controller.signal,
          onEvent: (streamEvent) => applyStreamEvent(streamEvent, assistantId),
        },
      );
    } catch (caught) {
      if (controller.signal.aborted) {
        markAssistantDone(assistantId);
        setStatus("idle");
        return;
      }
      const message = caught instanceof Error ? caught.message : "请求失败";
      setError(message);
      setStatus("error");
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId
            ? { ...item, content: item.content || message, status: "error" }
            : item,
        ),
      );
    } finally {
      abortRef.current = null;
    }
  }

  function applyStreamEvent(streamEvent: ChatStreamEvent, assistantId: string) {
    if (streamEvent.intent) {
      setLastIntent(streamEvent.intent);
    }

    if (streamEvent.type === "agent.started") {
      setStatus("streaming");
      return;
    }

    if (streamEvent.type === "agent.delta" && streamEvent.delta) {
      setStatus("streaming");
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId
            ? { ...item, content: item.content + streamEvent.delta, intent: streamEvent.intent ?? item.intent }
            : item,
        ),
      );
      return;
    }

    if (streamEvent.type === "agent.completed") {
      setStatus("done");
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId
            ? {
                ...item,
                content: streamEvent.answer || item.content,
                intent: streamEvent.intent ?? item.intent,
                status: "done",
              }
            : item,
        ),
      );
      return;
    }

    if (streamEvent.type === "agent.error") {
      const message = streamEvent.error || "服务端返回错误";
      setError(message);
      setStatus("error");
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId ? { ...item, content: item.content || message, status: "error" } : item,
        ),
      );
    }
  }

  function markAssistantDone(id: string) {
    setMessages((current) =>
      current.map((item) => (item.id === id ? { ...item, status: "done" } : item)),
    );
  }

  function stopStreaming() {
    abortRef.current?.abort();
  }

  function resetChat() {
    stopStreaming();
    setStatus("idle");
    setError("");
    setLastIntent("待识别");
    setMessages([
      {
        id: "welcome",
        role: "assistant",
        content: "新的会话已经准备好。输入你的学习目标或问题即可开始。",
        status: "done",
      },
    ]);
  }

  return (
    <main className="min-h-screen bg-[#f4f0e8] text-stone-950 lg:h-screen lg:overflow-hidden">
      <div className="mx-auto grid min-h-screen w-full max-w-7xl grid-cols-1 lg:h-screen lg:min-h-0 lg:grid-cols-[minmax(280px,320px)_minmax(0,1fr)]">
        <aside className="border-b border-stone-300 bg-[#efe7da] px-4 py-5 sm:px-5 lg:h-screen lg:overflow-y-auto lg:border-b-0 lg:border-r">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-emerald-700 text-white">
              <BrainCircuit size={22} />
            </div>
            <div>
              <h1 className="text-lg font-semibold">Learning Agent</h1>
              <p className="text-sm text-stone-600">学习工作台</p>
            </div>
          </div>

          <section className="mt-8 space-y-4">
            <div>
              <label className="text-sm font-medium text-stone-700" htmlFor="user-id">
                用户 ID
              </label>
              <input
                id="user-id"
                value={userId}
                onChange={(event) => setUserId(event.target.value)}
                className="mt-2 h-10 w-full rounded-md border border-stone-300 bg-white px-3 text-sm outline-none transition focus:border-emerald-700 focus:ring-2 focus:ring-emerald-700/15"
              />
            </div>
            <div>
              <label className="text-sm font-medium text-stone-700" htmlFor="session-id">
                会话 ID
              </label>
              <input
                id="session-id"
                value={sessionId}
                onChange={(event) => setSessionId(event.target.value)}
                className="mt-2 h-10 w-full rounded-md border border-stone-300 bg-white px-3 text-sm outline-none transition focus:border-emerald-700 focus:ring-2 focus:ring-emerald-700/15"
              />
            </div>
          </section>

          <section className="mt-7 rounded-lg border border-stone-300 bg-white/70 p-4">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm font-medium text-stone-700">运行状态</span>
              <StatusBadge status={status} label={statusText} />
            </div>
            <div className="mt-4 grid grid-cols-[auto_1fr] gap-x-3 gap-y-3 text-sm">
              <PanelLeft size={18} className="text-stone-500" />
              <span className="text-stone-700">接口：SSE 流式对话</span>
              <Sparkles size={18} className="text-stone-500" />
              <span className="text-stone-700">意图：{lastIntent}</span>
            </div>
          </section>

          <section className="mt-7">
            <h2 className="text-sm font-semibold text-stone-700">快速开始</h2>
            <div className="mt-3 space-y-2">
              {examples.map((example) => (
                <button
                  key={example}
                  type="button"
                  onClick={() => setInput(example)}
                  className="w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-left text-sm text-stone-700 transition hover:border-emerald-700 hover:text-emerald-800"
                >
                  {example}
                </button>
              ))}
            </div>
          </section>

          <button
            type="button"
            onClick={resetChat}
            className="mt-7 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-stone-300 bg-white px-3 text-sm font-medium text-stone-800 transition hover:border-stone-500"
          >
            <RotateCcw size={16} />
            重置当前页面
          </button>
        </aside>

        <section className="flex min-h-[70vh] min-w-0 flex-col lg:h-screen lg:min-h-0">
          <header className="flex min-h-16 flex-col gap-3 border-b border-stone-300 bg-[#fbfaf7] px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-5">
            <div className="min-w-0">
              <h2 className="text-base font-semibold">Agent 对话</h2>
              <p className="text-sm text-stone-600">制定计划、提问、练习与复盘都在这里完成。</p>
            </div>
            {status === "streaming" || status === "connecting" ? (
              <button
                type="button"
                onClick={stopStreaming}
                className="inline-flex h-9 items-center justify-center rounded-md bg-stone-900 px-3 text-sm font-medium text-white transition hover:bg-stone-700"
              >
                停止
              </button>
            ) : null}
          </header>

          <div className="min-h-[50vh] flex-1 overflow-y-auto px-4 py-5 sm:px-6 lg:min-h-0">
            <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
              {messages.map((message) => (
                <MessageBubble key={message.id} message={message} />
              ))}
              {error ? (
                <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-3 text-sm text-red-800">
                  <CircleAlert size={18} className="mt-0.5 shrink-0" />
                  <span>{error}</span>
                </div>
              ) : null}
            </div>
          </div>

          <form onSubmit={handleSubmit} className="border-t border-stone-300 bg-[#fbfaf7] px-4 py-4 sm:px-6">
            <div className="mx-auto flex w-full max-w-3xl flex-col gap-3 sm:flex-row sm:items-end">
              <textarea
                value={input}
                onChange={(event) => setInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                    event.currentTarget.form?.requestSubmit();
                  }
                }}
                placeholder="输入你的学习目标、问题或复盘内容"
                rows={3}
                className="min-h-24 w-full flex-1 resize-none rounded-lg border border-stone-300 bg-white px-4 py-3 text-sm leading-6 outline-none transition placeholder:text-stone-400 focus:border-emerald-700 focus:ring-2 focus:ring-emerald-700/15"
              />
              <button
                type="submit"
                disabled={!canSend}
                title="发送"
                className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-md bg-emerald-700 px-4 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:bg-stone-300 disabled:text-stone-500 sm:w-auto sm:min-w-24"
              >
                {status === "connecting" || status === "streaming" ? (
                  <Loader2 size={17} className="animate-spin" />
                ) : (
                  <Send size={17} />
                )}
                发送
              </button>
            </div>
          </form>
        </section>
      </div>
    </main>
  );
}

function StatusBadge({ status, label }: { status: string; label: string }) {
  const icon =
    status === "connecting" || status === "streaming" ? (
      <Loader2 size={14} className="animate-spin" />
    ) : status === "error" ? (
      <CircleAlert size={14} />
    ) : (
      <CheckCircle2 size={14} />
    );

  return (
    <span className="inline-flex h-7 items-center gap-1.5 rounded-full border border-stone-300 bg-white px-2.5 text-xs font-medium text-stone-700">
      {icon}
      {label}
    </span>
  );
}

function MessageBubble({ message }: { message: Message }) {
  const isUser = message.role === "user";

  return (
    <article className={`flex min-w-0 gap-2 sm:gap-3 ${isUser ? "justify-end" : "justify-start"}`}>
      {!isUser ? (
        <div className="mt-1 flex size-8 shrink-0 items-center justify-center rounded-md bg-emerald-700 text-white">
          <Bot size={17} />
        </div>
      ) : null}
      <div
        className={`max-w-[calc(100%-2.5rem)] rounded-lg border px-4 py-3 text-sm leading-6 shadow-sm sm:max-w-[min(720px,calc(100%-3rem))] ${
          isUser
            ? "border-stone-900 bg-stone-900 text-white"
            : "border-stone-300 bg-white text-stone-850"
        }`}
      >
        {message.intent ? (
          <div className="mb-2 text-xs font-medium text-emerald-700">意图：{message.intent}</div>
        ) : null}
        <div className="whitespace-pre-wrap break-words">
          {message.content || (message.status === "streaming" ? "正在思考..." : "")}
        </div>
      </div>
      {isUser ? (
        <div className="mt-1 flex size-8 shrink-0 items-center justify-center rounded-md bg-stone-900 text-white">
          <User size={17} />
        </div>
      ) : null}
    </article>
  );
}

export default App;
