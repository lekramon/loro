# Triage labels

| Role | Label | Meaning |
|---|---|---|
| Needs triage | `needs-triage` | Evaluate scope and behavior before implementation. |
| Needs information | `needs-info` | Required information is missing. |
| Ready for agent | `ready-for-agent` | Behavior and acceptance are sufficiently specified for implementation. |
| Ready for human | `ready-for-human` | Requires human implementation or external action. |
| Declined | `wontfix` | Will not be implemented. |

Execution is separate: `State: open`, `State: in-progress`, or `State: done`. Dependencies determine whether work can start, independent of its label. Feature documents may use `State: specified` to indicate prepared requirements without claiming implementation.
