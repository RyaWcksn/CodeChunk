# code-indexing

Chunk code (TypeScript / Go) into semantic units, embed with a local Ollama
model, and store in Chroma Cloud.

## Requirements

- [Bun](https://bun.com) >= 1.4
- [Ollama](https://ollama.com) running on `localhost:11434`
- Embedding model pulled: `ollama pull nomic-embed-text`
- A [Chroma Cloud](https://trychroma.com) tenant + database (or any Chroma server reachable by HTTP)

## Install

```bash
bun install
```

## Environment

Set before running the indexer. Either export in your shell, source an `.env`
file, or use a private file like `~/.code-indexing.env`.

| variable             | default                          | notes                                       |
| -------------------- | -------------------------------- | ------------------------------------------- |
| `OLLAMA_URL`         | `http://localhost:11434`         | Ollama server                               |
| `EMBED_MODEL`        | `nomic-embed-text`               | any model Ollama serves for embeddings      |
| `CHROMA_COLLECTION`  | `code_chunks`                    | collection name                             |
| `CHROMA_HOST`        | (uses Chroma Cloud default)      | override for self-hosted                    |
| `CHROMA_API_KEY`     | (required)                       | or `CHROMA_TOKEN`                           |
| `CHROMA_TENANT`      | `default`                        |                                             |
| `CHROMA_DATABASE`    | `default`                        |                                             |

Example shell setup:

```sh
export CHROMA_HOST=chroma.example.com
export CHROMA_API_KEY=ck-...
export CHROMA_TENANT=default
export CHROMA_DATABASE=dev
```

## Usage

Print chunks for a file:

```bash
bun run index.ts examples/user.ts
```

Index a file into Chroma (chunk → embed → upsert):

```bash
bun run index.ts index examples/user.ts
```

Index many files:

```bash
for f in src/**/*.ts; do bun run index.ts index "$f"; done
```

## Programmatic API

```ts
import { chunk, chunkFile, embed, indexFile } from "./index.ts";

chunkFile("foo.go");                       // Chunk[] for a file
chunk(src, "go");                          // Chunk[] from raw source
await embed(["hello", "world"]);           // number[][]
await indexFile("foo.go");                 // chunk + embed + upsert
```

## Examples

| file                        | what it shows                            |
| --------------------------- | ---------------------------------------- |
| `examples/chunk.ts`         | print chunks of one file                 |
| `examples/index.ts`         | index one or more files                  |
| `examples/query.ts`         | semantic search the collection           |
| `examples/export.ts`        | dump the collection to a JSON file       |
| `examples/user.ts`          | TypeScript sample for chunking demos     |
| `examples/user.go`          | Go sample for chunking demos             |
| `examples/order.go`         | complex Go (interfaces, generics, methods)|

Run any example with `bun run examples/<name>.ts`.

## How chunking works

`index.ts` walks the top-level named children of the parsed syntax tree.
TypeScript chunks unwrap a single `export_statement` so the exported
declaration itself becomes the unit (`export default class Foo` →
`class Foo`). Go chunks are emitted as-is — `function_declaration`,
`method_declaration`, `type_declaration`, etc. Both grammars use
`tree-sitter` so chunks align with real language structure instead of
blank lines.

## How embedding works

`embed()` POSTs to Ollama's `/api/embed` endpoint in batches of 32 (kept
under the model's 2048-token context). `OllamaEF` registers with Chroma
so query-time strings are embedded by the same model — no extra
dependency on `@chroma-core/default-embed`.

## Security

Credentials are read from environment only. Never hardcode `CHROMA_API_KEY`
or tokens into source.
