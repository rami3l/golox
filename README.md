# golox

[The Lox Programming Language](https://www.craftinginterpreters.com/the-lox-language.html) implemented in Go, based on the original `clox` implementation\*.

\* : For the tree-walking interpreter, see [`rami3l/dolores`](https://github.com/rami3l/dolores).

---

## Contents

- [golox](#golox)
  - [Contents](#contents)
  - [Features](#features)
  - [Try it out!](#try-it-out)

---

## Features

- [x] Lexer
- [x] Pratt parser & bytecode compiler
- [x] Bytecode VM
- [x] Basic types
- [x] Floating point arithmetic
- [x] Logic expressions
- [x] Control flow
- [x] Jumps: `break`/`continue`\*\*
- [x] Functions
- [x] Classes
- [x] Instances
- [x] Instance methods
  - [x] `this`
  - [x] Initializers
- [x] Inheritance
  - [x] `super`

\*\* : Extension

## Try it out!

With the latest [Go toolchain](https://go.dev/dl) installed:

To refresh generated source files:

```sh
go generate ./...
```

To run:

```sh
go run main.go
```

To run with debug info:

```sh
go generate ./... && go run -tags DEBUG main.go -v=debug
```
