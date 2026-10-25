# Design notes / 已实施设计记录

这里保留两项结构性改动的设计背景与落地结果。两项均已完成，当前没有对应的
待办项。

---

<a id="handler-boundaries"></a>
## 1. Handler 接口窄化

**状态：已实施**（`refactor: narrow public handler dependencies`）

### 结果

- `internal/store/read.go` 新增按用途拆分的窄接口：
  - `PostReader`、`PostWriter`、`PostMetrics`
  - `TaxonomyReader`
  - `CommentReader`、`CommentWriter`
  - `NoteReader`
  - `OptionReader`
- `store.Store` 继续作为组合接口，内部嵌入这些窄接口；`SQLiteStore` 不变。
- `internal/handler/deps.go` 新增 `handler.Deps`，公共页面、feed 和 JSON API
  只接收这个装配结构：

```go
type Deps struct {
    Posts         store.PostReader
    PostWrites    store.PostWriter
    Views         store.PostMetrics
    Taxonomy      store.TaxonomyReader
    Comments      store.CommentReader
    CommentWrites store.CommentWriter
    Notes         store.NoteReader
    Options       store.OptionReader
}
```

- 所有 `internal/handler` 中的公开 handler 不再直接依赖 `store.Store`。
- `handler.NewDeps(db)` 在 `internal/server` 中统一装配。
- `internal/admin` 继续使用完整 `store.Store`：后台本身就是全部管理能力的
  入口，按子域拆分不会降低实际权限，反而会增加装配噪音。
- 新增 `TestRobotsWithNarrowDeps`，只提供一个 `OptionReader` fake 即可测试
  `Robots`，证明 handler 不再要求真实 SQLite。

### 边界

- 公共 handler 的依赖已经收窄；store 实现仍是完整接口。
- 后续如果后台需要独立拆分，可以按 `PostStore` / `TaxonomyStore` /
  `CommentStore` 等组合接口继续切分。

---

<a id="inline-math"></a>
## 2. 行内 `$...$` 公式

**状态：已实施**（`feat: add opt-in inline dollar math`）

### 结果

- `posts` 新增 `math INTEGER NOT NULL DEFAULT 0`（迁移 `002_post_math.sql`）。
- `model.Post.Math`、`content.PostInput.Math` 和 JSON API 均支持该字段。
- 后台编辑器新增「启用行内 $ 公式」开关。
- 只有 `math=true` 的文章才启用行内美元公式；导入的历史文章默认关闭。
- 行内公式在 Goldmark 解析阶段转换为 `\(...\)`：
  - 避免 `$a_i$` 中的下划线被 Markdown 当成强调；
  - 避免客户端把正文里的货币写法 `$5 ... $10` 当成公式；
  - 代码块和行内代码中的 `$` 不受影响；
  - 模板不再注册客户端 `$` 分隔符。
- 块级 `$$...$$`、`\[...\]` 和显式 `\(...\)` 始终可用。
- 后台预览接口使用同一套 `RenderPost` 逻辑。

### 验收覆盖

- 历史 PHP/shell 文章（`$var`、`$_REQUEST`）渲染不变。
- 开启 math 后 `$x^2$`、`$a_i + b^2$` 正常转换。
- 货币写法、转义美元、未配对美元不会启用 KaTeX。
- `pre` / `code` 中的 `$` 不被转换。
- HTML 格式文章中的行内公式同样支持。
- store、content、admin、handler 和模板测试均覆盖。
