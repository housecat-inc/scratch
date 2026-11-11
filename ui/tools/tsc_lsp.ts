// oxlint-disable effecttsgo/node-builtin-import -- The CLI and LSP transport require Node file descriptors, synchronous reads, raw Buffers, and process pipes.
import * as Arr from "effect/Array";
import * as Console from "effect/Console";
import * as Effect from "effect/Effect";
import * as Predicate from "effect/Predicate";
import type { Readable, Writable } from "node:stream";
import { required } from "../dom/required";
type Message = Record<PropertyKey, unknown>;
function object(value: unknown): Message {
  return Predicate.isObject(value) ? value : {};
}
// Runs the compiler's language server for ui/tsc.sh --lsp:
//
//   node tools/tsc_lsp.ts <compiler> --lsp --stdio
//
// The server publishes a file's diagnostics only in answer to
// textDocument/diagnostic, and Claude Code never sends one, so for a client
// that does not pull diagnostics this pulls them after each open and change,
// and for every open file when the server marks them stale, and pushes each
// answer. A client that pulls them itself gets the server's messages untouched.
import { spawn } from "node:child_process";

const [compiler, ...args] = process.argv.slice(2);
if (!compiler) throw new Error("Missing compiler executable");
const server = spawn(compiler, args, { stdio: ["pipe", "pipe", "inherit"] });
// oxlint-disable-next-line unicorn/no-process-exit -- The CLI must exit immediately with the compiler status and close its inherited stdio pipes.
server.on("close", (code) => process.exit(code ?? 1));
process.stdin.on("end", () => server.stdin.end());

const readLspMessages = (stream: Readable, handle: (message: Message) => void) => {
  let buffer = Buffer.alloc(0);
  stream.on("data", (chunk) => {
    buffer = Buffer.concat([buffer, chunk]);
    for (;;) {
      const headerEnd = buffer.indexOf("\r\n\r\n");
      if (headerEnd < 0) return;
      const length = Number(
        /[cC][oO][nN][tT][eE][nN][tT]-[lL][eE][nN][gG][tT][hH]: *(\d+)/u.exec(
          buffer.subarray(0, headerEnd).toString(),
        )?.[1],
      );
      const bodyEnd = headerEnd + 4 + length;
      if (buffer.length < bodyEnd) return;
      handle(object(JSON.parse(buffer.subarray(headerEnd + 4, bodyEnd).toString("utf8"))));
      buffer = buffer.subarray(bodyEnd);
    }
  });
};
const sendLspMessage = (stream: Writable, message: Message) => {
  const body = Buffer.from(JSON.stringify({ jsonrpc: "2.0", ...message }), "utf8");
  stream.write(Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`), body]));
};
const publishDiagnostics = (uri: string, diagnostics: unknown[]) =>
  sendLspMessage(process.stdout, {
    method: "textDocument/publishDiagnostics",
    params: { uri, diagnostics },
  });

let clientPulls = true;
const openUris = new Set<string>();
const latestRequestByUri = new Map<string, string>();
const uriByRequestId = new Map<unknown, string>();
let lastRequestId = 0;
const requestDiagnostics = (uri: string) => {
  const id = `tsc-lsp:${++lastRequestId}`;
  latestRequestByUri.set(uri, id);
  uriByRequestId.set(id, uri);
  sendLspMessage(server.stdin, {
    id,
    method: "textDocument/diagnostic",
    params: { textDocument: { uri } },
  });
};

readLspMessages(process.stdin, (message) => {
  if (message.method === "initialize") {
    const params = object(message.params);
    const capabilities = object(params.capabilities);
    clientPulls = Boolean(object(capabilities.textDocument).diagnostic);
    if (!clientPulls) {
      capabilities.workspace = {
        ...object(capabilities.workspace),
        diagnostics: { refreshSupport: true },
      };
      params.capabilities = capabilities;
    }
  }
  sendLspMessage(server.stdin, message);
  if (clientPulls) return;
  const uri = object(object(message.params).textDocument).uri;
  if (typeof uri !== "string") return;
  switch (message.method) {
    case "textDocument/didOpen":
      openUris.add(uri);
      requestDiagnostics(uri);
      break;
    case "textDocument/didChange":
      requestDiagnostics(uri);
      break;
    case "textDocument/didClose":
      openUris.delete(uri);
      latestRequestByUri.delete(uri);
      publishDiagnostics(uri, []);
      break;
  }
});

readLspMessages(server.stdout, (message) => {
  if (!clientPulls && uriByRequestId.has(message.id)) {
    const uri = required(uriByRequestId.get(message.id));
    uriByRequestId.delete(message.id);
    if (latestRequestByUri.get(uri) !== message.id) return;
    if (message.error !== undefined) {
      Effect.runSync(
        Console.error(`tsc-lsp: diagnostics for ${uri}: ${String(object(message.error).message)}`),
      );
    } else if (object(message.result).kind === "full") {
      const items = object(message.result).items;
      publishDiagnostics(uri, Arr.isArray(items) ? items : []);
    }
    return;
  }
  if (!clientPulls && message.method === "workspace/diagnostic/refresh") {
    sendLspMessage(server.stdin, { id: message.id, result: null });
    for (const uri of openUris) requestDiagnostics(uri);
    return;
  }
  sendLspMessage(process.stdout, message);
});
