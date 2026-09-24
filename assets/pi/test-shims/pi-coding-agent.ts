// Test-only declaration surface; production keeps the official Pi import.
export const VERSION = "test";

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
type SessionContext = {
  mode: string;
  ui: {
    setHeader(factory: (tui: unknown, theme: Theme) => Header): void;
  };
};

export interface ExtensionAPI {
  exec(
    command: string,
    args: string[],
    options?: { timeout?: number },
  ): Promise<{ stdout: string; stderr: string; code: number; killed: boolean }>;
  registerTool(tool: unknown): void;
  on(
    event: "session_start",
    handler: (event: unknown, ctx: SessionContext) => unknown,
  ): void;
  on(
    event: "before_agent_start",
    handler: (event: { systemPrompt: string }) => unknown,
  ): void;
  on(event: "tool_call", handler: (event: ToolCallEvent) => unknown): void;
}
