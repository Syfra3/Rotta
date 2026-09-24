// Test-only declaration surface; production keeps the official Pi import.
export const VERSION = "0.87.1";
export function formatSize(bytes: number) {
  return bytes >= 1024 ? `${(bytes / 1024).toFixed(1)} KB` : `${bytes} B`;
}

export interface ToolDefinition<TParams = unknown, TDetails = unknown, TState = unknown> {
  name: string;
  label: string;
  description: string;
  parameters: TParams;
  execute: (...args: any[]) => Promise<any>;
  renderCall?: (...args: any[]) => any;
  renderResult?: (...args: any[]) => any;
  [key: string]: unknown;
}

const bashParameters = Object.freeze({ kind: "bash-schema" });
const editParameters = Object.freeze({ kind: "edit-schema" });
export function createBashToolDefinition(cwd: string): ToolDefinition {
  return {
    name: "bash",
    label: "bash",
    description: "built-in bash",
    parameters: bashParameters,
    cwd,
    execute: async () => ({ content: [], details: undefined }),
    renderCall: () => ({ render: () => ["native bash call"], invalidate() {} }),
    renderResult: () => ({ render: () => ["native bash result"], invalidate() {} }),
  };
}
export function createEditToolDefinition(cwd: string): ToolDefinition {
  return {
    name: "edit",
    label: "edit",
    description: "built-in edit",
    parameters: editParameters,
    cwd,
    execute: async () => ({ content: [], details: undefined }),
    renderCall: () => ({ render: () => ["native edit call"], invalidate() {} }),
    renderResult: () => ({ render: () => ["native edit result"], invalidate() {} }),
  };
}

export interface Theme {
  bold(text: string): string;
  fg(name: string, text: string): string;
}

type Header = {
  render(width: number): string[];
  invalidate(): void;
};
type ToolCallEvent = {
  toolName: string;
  input: unknown;
};
type ToolExecutionStartEvent = {
  type: "tool_execution_start";
  toolCallId: string;
  toolName: string;
  args: unknown;
};
type ToolExecutionUpdateEvent = {
  type: "tool_execution_update";
  toolCallId: string;
  toolName: string;
  args: unknown;
  partialResult: unknown;
};
type ToolExecutionEndEvent = {
  type: "tool_execution_end";
  toolCallId: string;
  toolName: string;
  result: unknown;
  isError: boolean;
};
type ToolResultEvent = {
  type: "tool_result";
  toolCallId: string;
  toolName: string;
  input: Record<string, unknown>;
  content: unknown[];
  details: unknown;
  isError: boolean;
  usage?: unknown;
};
type ToolResultEventResult = {
  content?: unknown[];
  details?: unknown;
  isError?: boolean;
  usage?: unknown;
};
type SessionContext = {
  mode: string;
  ui: {
    setHeader(factory: (tui: unknown, theme: Theme) => Header): void;
  };
};

export interface ExtensionAPI {
  events: {
    emit(name: string, payload: unknown): void;
  };
  exec(
    command: string,
    args: string[],
    options?: { timeout?: number },
  ): Promise<{ stdout: string; stderr: string; code: number; killed: boolean }>;
  registerTool(tool: unknown): void;
  registerShortcut(
    shortcut: string,
    options: {
      description?: string;
      handler: (ctx: SessionContext) => Promise<void> | void;
    },
  ): void;
  on(
    event: "session_start",
    handler: (event: unknown, ctx: SessionContext) => unknown,
  ): void;
  on(
    event: "before_agent_start",
    handler: (event: { systemPrompt: string }) => unknown,
  ): void;
  on(event: "session_shutdown", handler: (event: unknown) => unknown): void;
  on(event: "tool_call", handler: (event: ToolCallEvent) => unknown): void;
  on(
    event: "tool_result",
    handler: (
      event: ToolResultEvent,
      ctx: SessionContext,
    ) => Promise<ToolResultEventResult | void> | ToolResultEventResult | void,
  ): void;
  on(
    event: "tool_execution_start",
    handler: (event: ToolExecutionStartEvent, ctx: SessionContext) => unknown,
  ): void;
  on(
    event: "tool_execution_update",
    handler: (event: ToolExecutionUpdateEvent, ctx: SessionContext) => unknown,
  ): void;
  on(
    event: "tool_execution_end",
    handler: (event: ToolExecutionEndEvent, ctx: SessionContext) => unknown,
  ): void;
}
