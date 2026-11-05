# agent/v2

多轮工具调用 agent 的最小执行库。一个 `Runner` 持有模型与工具，一次 `Run` 循环「调用模型 → 执行工具 → 再调用模型」，
直到模型给出最终回答、达到轮数上限或预算超限。本包只做编排，不关心业务：状态、提示词、持久化都由调用方提供。

依赖 `github.com/cloudwego/eino` 的 `ToolCallingChatModel` 与 `schema`。

## 快速开始

```go
runner, err := agent.NewStatelessRunner(ctx, agent.RunnerConfig{
	Name:  "assistant",
	Model: chatModel,
	Tools: []agent.Tool{&EchoTool{}},
	MaxRound: 8,
})
if err != nil {
	return err
}

session := agent.NewStatelessSession()
session.System = "You are a helpful assistant."
session.Append(schema.UserMessage("帮我查一下 MK-801 的半数致死量"))

res, err := runner.Run(ctx, session, agent.StatelessRunConfig{})
if err != nil {
	return err
}

fmt.Println(res.Final.Content) // 最终回答
fmt.Println(res.Rounds, res.ToolCalls, res.Usage)
```

- `Runner` 构造后不可变，可以跨会话并发共享；`Session` 只允许单 goroutine 持有。
- `Run` 是非流式；需要增量输出用 `RunStream`，内容/推理增量通过 `Hooks` 的 `OnContentDelta` / `OnReasoningDelta` 拿到。
- 有业务状态时用 `agent.NewRunner[MyState](ctx, cfg)` + `agent.NewSession(state)`，`State` 在 hooks 里通过 `ctx.Session.State` 读写。

## 定义工具

工具就是 eino 的 `InvokableTool`，`agent.Tool` 是它的别名：

```go
type EchoTool struct{}

func (t *EchoTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "echo", Desc: "原样返回参数"}, nil
}

func (t *EchoTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	return args, nil
}
```

工具名必须非空且唯一，`NewRunner` 与 `Run` 都会校验；工具集按名字排序后绑定给模型，保证同一组工具生成的请求稳定。

工具的注册来源有三层，后者覆盖前者：

| 来源 | 用途 |
|---|---|
| `RunnerConfig.Tools` | Runner 的静态工具集 |
| `RunConfig.ExtraTools` | 本次运行追加，重名直接报错 |
| `RoundPlan.Tools` / `RoundPlan.NoTools` | 某一轮整体替换或禁用（在 `OnRoundStart` 里改） |
| `RunConfig.WithoutTools` | 本次运行完全不用工具 |

## 工具执行策略（`ToolPolicy`）

```go
runner, _ := agent.NewRunner[struct{}](ctx, agent.RunnerConfig{
	Model:  chatModel,
	Tools:  tools,
	Policy: agent.ToolPolicy{
		Serial:         true,             // 串行执行，默认 false（并行）
		MaxConcurrency: 4,                // 并行度上限，0 表示不限制；串行模式忽略
		Timeout:        30 * time.Second, // 单个工具超时，0 表示跟随 ctx
		MaxResultChars: 8000,             // 单个工具结果截断，0 表示不截断
	},
})
```

也可以只对某一轮生效：在 `OnRoundStart` 里设置 `rc.Plan.ToolPolicy = &agent.ToolPolicy{Serial: true}`。

### 并行（默认）

同一轮的多个工具调用并发执行，`MaxConcurrency` 限制并发度。适合只读、互不依赖的工具（检索、读取文件、并行子任务）。

### 串行

`Serial: true` 时按模型给出的调用顺序逐个执行，同一时刻只有一个工具在跑。适合有副作用的工具（写文件、改状态、有先后依赖的命令）。

两种模式共享同一条健壮性约定：**单个工具失败不会阻塞同一轮的其他工具**。
失败被记录在该条 `ToolResult.Error` 上，作为 `tool` 消息回给模型，其余调用照常执行；串行模式下后续工具依然会跑完。
只有 hook 返回错误才会让整轮失败（此时工具已经执行完，错误在全部结束后返回）。

### 结果与失败形态

`executeTools` 返回的 `[]ToolResult` 永远按**模型调用顺序**排列，与完成顺序无关，可直接转成 `tool` 消息。

| 情况 | `ToolResult.Result` |
|---|---|
| 成功 | 工具输出，超过 `MaxResultChars` 追加 `...[tool result truncated]` |
| 工具返回错误 | `tool call failed: <err>`，`Error` 非空 |
| 工具名不存在 | `tool call failed: tool <name> not found` |
| 工具 panic | `tool call panic: <v>`（栈打到日志，进程不受影响） |
| 工具超时 | `tool call failed: context deadline exceeded` 一类，取决于工具对 ctx 的处理 |

`ToolResult.Duration` 是单个工具的实际耗时；`Result.ToolCalls` 是本轮累计的工具调用条数。

## 轮次、停止与末轮

- `RunnerConfig.MaxRound` 默认 20，`RunConfig.MaxRound` 可按本次运行覆盖。
- 每轮先调模型；没有工具调用即结束（`StopFinal`），有工具调用则执行后进入下一轮。
- 达到上限返回 `ErrMaxRounds`。
- 未传 `Hooks` 时使用 `FinalRoundHook`：最后一轮注入「直接输出结果、不要再调工具」的指令并解绑工具；
  自定义 hooks 时若仍需要该行为，把 `FinalRoundHook` 嵌进自己的 hooks。
- `RoundPlan.Inject` 可在某轮临时插入消息（不写入 `Session`），`rc.Stop = true` 可在 `OnRoundEnd` 主动收尾（`StopEarly`）。

## Hooks

嵌入 `agent.NoopHooks[S]` 只覆写关心的阶段：

| 回调 | 时机 | 能否影响执行 |
|---|---|---|
| `OnRunStart` / `OnRunEnd` | 整个运行前后 | `OnRunEnd` 拿到 `Result` 与错误 |
| `OnRoundStart` | 每轮调模型前 | 可改 `RoundPlan`（工具、模型、注入消息、`ToolPolicy`） |
| `OnRoundComplete` | 一轮结束 | 可读 `RoundResult` |
| `OnRoundEnd` | 一轮彻底结束 | 可置 `Stop` |
| `OnToolCallStart` | 本轮全部工具执行前，按调用顺序 | 返回错误会终止本轮（工具尚未执行） |
| `OnToolCallEnd` | 每个工具结束后 | 并行模式下会在多个 goroutine 中并发调用，注意加锁 |
| `OnContentDelta` / `OnReasoningDelta` 等 | 流式输出期间 | 仅 `RunStream` 有增量 |

## 预算

`RunConfig.Budget` 是进程内的运行护栏，并发安全，可跨子运行共享：

```go
budget := agent.NewRunBudget(200_000, 40) // 0 表示该维度不限制
res, err := runner.Run(ctx, session, agent.StatelessRunConfig{Budget: budget})
if errors.Is(err, agent.ErrBudgetExceeded) {
	// 用 res.Usage 自行结算
}
```

本轮工具调用条数在执行前校验，token 用量在模型返回后结算；任一维度超限立即返回错误。它不持久化、不区分用户，用户级配额应在应用层预检查。

## 错误

| 错误 | 含义 |
|---|---|
| `ErrMaxRounds` | 超过最大轮数 |
| `ErrEmptyResponse` | 模型返回空消息 |
| `ErrBudgetExceeded` | token 或工具调用次数超预算 |
| `ErrContextOverflow` | 上下文超出预算，预留给调用方在压缩场景使用 |

模型/工具相关错误统一包成 `pkg/errors` 的 `ErrLLM` / `ErrParams`，可直接 `errors.Is` / `errors.As`。
