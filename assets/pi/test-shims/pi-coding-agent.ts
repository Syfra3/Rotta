// Test-only declaration surface; production keeps the official Pi import.
export interface ExtensionAPI {
  registerTool(tool: unknown): void;
  on(event: string, handler: unknown): void;
}
