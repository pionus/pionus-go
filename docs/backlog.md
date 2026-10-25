# Backlog / 待办设计记录

这里记录已经明确、但尚未实施的改动。每一项包含现状、为什么值得做、可选方案、
建议方案和验收标准，方便后续直接开工。

---

<a id="handler-boundaries"></a>
## 1. Handler 接口窄化（降低 store 耦合）

### 现状

- 所有公开 handler 都接收完整 `store.Store`：
  - `internal/handler/page.go`：`Index(db store.Store)`、`ArticleList(db)`、
    `ArticleDetail(db, guard)`、`CategoryArchive(db)`、`DateArchive(db)`、
    `Search(db)`、`NotesPage(db)` 等。
  - `internal/handler/post.go`：JSON API 的增删改查。
  - `internal/handler/feed.go`、`site.go`、`comments.go`、`notes.go` 同样。
- 后台 `internal/admin` 也大量使用完整 `store.Store`。
- `store.Store` 由 `PostStore + TaxonomyStore + CommentStore + UserStore +
  MediaStore + NoteStore + OptionStore` 组合而成，接近 50 个方法。
- 测试必须启动真实 SQLite（如 `internal/handler/post_test.go`），无法只给
  handler 一个内存 fake。

### 问题

1. **依赖过宽**：一个只读文章列表的 handler 理论上只需要
   `ListPosts/GetOption`，实际却拿到了删除用户、写 media 等全部能力。
2. **测试成本高**：所有 handler 测试都依赖 SQLite 和迁移。
3. **改动传播大**：`Store` 接口新增一个方法，所有实现/测试都要跟着改。
4. **职责不清**：读写接口混在一起，公共页面天然能调用写方法。

### 建议方案

按使用场景拆成窄接口，保留 `store.Store` 作为组合接口用于装配：

```go
// internal/store/read.go（示意）
type PostReader interface {
    GetPost(slug string) (*model.Post, error)
    ListPosts(f PostFilter) ([]*model.Post, int, error)
    AdjacentPosts(slug string) (*model.Post, *model.Post, error)
}

type TaxonomyReader interface {
    GetCategory(slug string) (*model.Category, error)
    GetTag(slug string) (*model.Tag, error)
    ListCategories() ([]*model.Category, error)
    ListTags() ([]*model.Tag, error)
}

type CommentReader interface {
    ListComments(f CommentFilter) ([]*model.Comment, int, error)
}

type NoteReader interface {
    ListNotes(f NoteFilter) ([]*model.Note, int, error)
    GetNote(id int64) (*model.Note, error)
}

type OptionReader interface {
    GetOption(name string) (string, error)
    AllOptions() (map[string]string, error)
}
```

- 写接口单独定义（`PostWriter`、`CommentWriter`），只给后台/API。
- 在 `internal/handler` 增加一个装配结构，避免每个构造函数都传 5 个参数：

```go
type Deps struct {
    Posts    store.PostReader
    Taxonomy store.TaxonomyReader
    Comments store.CommentReader
    Notes    store.NoteReader
    Options  store.OptionReader
}
```

- 公共 handler 改为接收 `Deps`；API/后台按需接收写接口。
- `store.Store` 继续嵌入所有窄接口，`SQLiteStore` 不变。
- 测试可以用 fake reader，不必每次建库。

### 迁移顺序

1. 先补窄接口和 `Deps`，不改调用方（保持编译）。
2. 逐个改 `page.go` / `site.go` / `comments.go` / `notes.go` / `feed.go`。
3. 再改 `post.go`（API）和 `admin`。
4. 最后把测试从真实 DB 切到 fake，保留少量 store 集成测试。

### 验收标准

- 公开 handler 不再直接依赖 `store.Store`。
- 每个 handler 只声明自己实际使用的方法。
- `go test ./...` 通过；至少有一个 handler 测试使用 fake store。
- 行为、URL、模板数据完全不变。

### 风险 / 备注

- 改动面大但纯结构，建议在功能冻结后做一次，单独提交。
- `baseData` 同时读 options 和 pages，需要 `OptionReader + PostReader`。
- `IncrementPostViews` 是写操作，但公共详情页需要，可以放在 `PostReader`
  或单独的 `PostMetrics` 接口。

---

<a id="inline-math"></a>
## 2. 行内 `$...$` 公式

### 现状

- 服务端只检测/加载 KaTeX 的三种分隔符：
  - `$$...$$`（块级）
  - `\(...\)`（行内）
  - `\[...\]`（块级）
- 见 `internal/content/render.go` 的 `hasMath` 和
  `web/theme/pages/post.html` 的 `renderMathInElement` 配置。
- `$x$` 目前不会渲染，会原样显示。

### 为什么暂时不支持

线上 Typecho 数据里有大量 PHP/shell 变量，例如：

- `$Php2Html_FileUrl = $`
- `$sql = "select username,password from admin ..."`
- `$__PNAME_V$`、`$CHICAGO$`
- shell prompt：`$`、`#`
- 代码块中的 `$_REQUEST`、`$this` 等

如果直接启用 `$` 分隔符，KaTeX 会把成对的美元符号误判成公式，破坏代码类文章。
KaTeX 的 `ignoredTags: ['pre','code']` 只能保护代码块，保护不了正文里的 PHP 变量。

### 可选方案

**方案 A：保持现状（只支持 `$$` / `\(` / `\[`）**

- 优点：零误判、零改动。
- 缺点：作者写 `$x$` 不生效，需要记住 `\(x\)`。

**方案 B：仅对显式开启 math 的文章支持 `$`（推荐）**

- 给 `posts` 增加 `math` 布尔字段（或 `math_mode`）。
- 后台编辑器增加「启用行内 $ 公式」开关。
- 只有 `math=true` 的文章加载带 `$` 分隔符的 KaTeX 配置；其余文章保持现状。
- 导入的旧文章默认 `false`，避免历史 PHP 文章被误判。
- 未来写数学文章时手动打开，语义明确。

**方案 C：按 Markdown AST 判断**

- 用 Goldmark AST 跳过 `CodeSpan` / `CodeBlock`，只处理文本节点中的 `$...$`。
- 可以进一步要求内容包含 LaTeX 特征（`\`、`^`、`_`、`{`、`=`、`\frac`
  等），或者 `$` 两侧不能是字母/数字。
- 优点：不用改数据模型。
- 缺点：正文里的 PHP 变量仍可能误判；规则难解释，边界情况多。

**方案 D：只对 Markdown 文章启用 `$`**

- 旧 HTML 文章不受影响，但 Markdown 文章里仍可能有 shell/PHP 示例。
- 风险比方案 B 高。

### 建议

采用 **方案 B**：

1. 迁移新增 `posts.math INTEGER NOT NULL DEFAULT 0`。
2. `model.Post.Math bool`，store 读写。
3. 后台编辑器加 checkbox「行内 $ 公式」。
4. `content.RenderPost` 的 `HasMath` 逻辑：
   - 始终识别 `$$` / `\(` / `\[`。
   - `post.Math == true` 时才把 `$` 加入 KaTeX delimiters。
5. 模板根据 `post.Math` 输出不同的 `renderMathInElement` 配置。
6. 导入器默认 `math=0`；不自动猜测。
7. 文档补充写法约定：数学文章用 `$...$`/`$$...$$`，代码文章保持关闭。

### 验收标准

- 历史 PHP/shell 文章（含 `$var`）渲染结果不变。
- 开启 math 的文章：
  - `$x^2$` 行内渲染；
  - `$$...$$` 块级渲染；
  - `code` / `pre` 内的 `$` 不渲染。
- 转义美元、货币写法（如 `\$5`、`$5`）不误判。
- 新增 store/content/handler 测试覆盖以上场景。

### 风险 / 备注

- 若未来编辑器支持粘贴 LaTeX，建议同时在预览接口里使用同一套分隔符配置。
- KaTeX 资源已经自托管在 `web/theme/assets/katex/`，不需要新增外部依赖。
