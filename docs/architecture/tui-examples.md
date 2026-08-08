# TUI composition patterns

DiscRescue follows the current Bubble Tea examples in the upstream `examples/`
directory. The examples are implementation references, not a second product
architecture.

| Example | DiscRescue use |
| --- | --- |
| `list-fancy` / `list-simple` | Drive, action, resume, and history navigation use Bubbles list models with a compact delegate and component-owned cursor. |
| `textinputs` / `isbn-form` | Output folder and filename use focused Bubbles text inputs; only the focused input receives editing messages. |
| `progress-download` | Recovery progress is a ratio of durable pass coverage to the active target and is rendered by Bubbles progress. |
| `composable-views` | Loading spinner and page-specific controls are updated as child models and their commands are returned to Bubble Tea. |
| `tabs` | Lip Gloss borders, semantic accents, and composed rectangles establish grouping without dense telemetry. |
| `views` | Page routing remains explicit, while DiscRescue keeps recovery state owned by the coordinator. |

The visual treatment adapts these patterns to optical recovery: a dark surface,
violet-to-magenta progress, muted empty rails, bright selected rows, rounded
shell framing, and explicit text/markers for deferred and unreadable sectors.
Color is never the sole status signal.

Sources:

- <https://github.com/charmbracelet/bubbletea/tree/main/examples>
- <https://github.com/charmbracelet/bubbletea/tree/main/examples/list-fancy>
- <https://github.com/charmbracelet/bubbletea/tree/main/examples/textinputs>
- <https://github.com/charmbracelet/bubbletea/tree/main/examples/progress-download>
- <https://github.com/charmbracelet/bubbletea/tree/main/examples/composable-views>
- <https://github.com/charmbracelet/bubbletea/tree/main/examples/tabs>
