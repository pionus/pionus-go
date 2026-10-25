# Plugins and events

Pionus uses **compile-time plugins**. A plugin is a Go package built into the
binary; there is no runtime `.so` loading, plugin installer or sandbox.

This is deliberate: plugins are trusted code, deployment stays a single
binary, and the contract remains type-safe.

## Adding a plugin

1. Create a package under `plugins/<id>`.
2. Implement `plugin.Plugin` and optionally the lifecycle/event interfaces.
3. Add its factory to `plugins.Factories()`.
4. Enable it in `config.yaml`:

```yaml
plugins:
  enabled: [eventlog]
```

`plugins/eventlog` is a small reference implementation.

## Contract

```go
type Manifest struct {
    ID          string
    Name        string
    Version     string
    APIVersion  int
    Description string
}

type Plugin interface {
    Manifest() Manifest
}
```

Optional lifecycle interfaces:

```go
type Initializer interface {
    Init(context.Context, *Env) error
}

type Starter interface {
    Start(context.Context) error
}

type Stopper interface {
    Stop(context.Context) error
}
```

`Init` runs before the server starts listening and may register routes.
`Start` runs after initialization; background workers belong there.
`Stop` runs in reverse plugin order during shutdown.

`plugin.Env` exposes:

- `Config`: the full application config
- `Store`: the full store (compile-time plugins are trusted)
- `App`: the arry application, for route registration
- `Log`: a logger
- `Settings`: the plugin's section from `plugins.config`

Plugins can decode their own settings:

```go
var cfg struct {
    URL    string   `yaml:"url"`
    Events []string `yaml:"events"`
}
if err := env.DecodeSettings(&cfg); err != nil {
    return err
}
```

Secrets should come from environment variables rather than YAML.

## Events

Events are defined in `internal/events`:

```go
type Event struct {
    ID         string
    Type       Type
    Version    int
    OccurredAt time.Time
    Source     string
    Data       any
}
```

Current event types:

| Event | Payload |
|---|---|
| `post.created` | `events.PostData` |
| `post.updated` | `events.PostData` |
| `post.deleted` | `events.PostData` |
| `comment.created` | `events.CommentData` |
| `comment.status_changed` | `events.CommentStatusData` |
| `comment.deleted` | `events.DeletedData` |
| `note.created` | `events.NoteData` |
| `note.updated` | `events.NoteData` |
| `note.deleted` | `events.DeletedData` |

Source values: `web`, `api`, `admin`, `system`.

A plugin consumes events by implementing `events.Handler`:

```go
func (p *Plugin) HandleEvent(ctx context.Context, event events.Event) error {
    switch event.Type {
    case events.PostCreated:
        post := event.Data.(events.PostData).Post
        // notify, index, enqueue, ...
    }
    return nil
}
```

`Handler` methods are called **after** the database write commits. The manager
uses a 3-second timeout per plugin and does not fail the HTTP request when a
plugin returns an error or panics; failures are logged.

A plugin handler must:

- honor `context.Context`
- be safe for concurrent event delivery
- not mutate event payloads (they may be shared with the caller)
- keep slow work out of the request path; use its own queue/worker
- treat payloads as private: comment events contain author email and post
  events contain full content, so do not log or forward them blindly

Import jobs do not emit per-post events. A future import-completed event can be
added without replaying thousands of individual notifications.

## Routes

A plugin may register namespaced routes during `Init`:

```go
env.App.Router().Get("/plugins/my-plugin/status", func(ctx arry.Context) {
    ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
})
```

Use `/plugins/<id>` or `/api/plugins/<id>`. Do not register `/`, `/article`,
`/admin` or other core paths.

There is currently no plugin-specific admin router or admin navigation API.
Admin extensions will be a separate phase.

## Configuration

```yaml
plugins:
  enabled:
    - eventlog
  # Optional plugin-specific settings:
  # config:
  #   eventlog: {}
```

Environment override:

```bash
PIONUS_PLUGINS_ENABLED=eventlog
```

Startup rules:

- an enabled plugin must be registered
- IDs must be unique and match `[a-z0-9][a-z0-9-]*`
- `APIVersion` must match the host
- initialization/startup failure stops the server
- disabled plugins are not initialized or started

## Current limitations

- no runtime plugin loading or installation
- no sandbox
- no plugin-specific admin pages/settings UI
- no plugin data migration API
- no filter hooks that can rewrite content
- no persistent event queue; a webhook plugin must provide its own

The recommended next extension is a webhook plugin built on this event model.
