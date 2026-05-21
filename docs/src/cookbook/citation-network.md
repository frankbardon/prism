# Cookbook: drawing a citation network

The `network` mark lays out a node-link diagram via Fruchterman-Reingold
force-directed relaxation. The layout runs at encode time and is
deterministic for a fixed `mark.seed`, so SVG output is byte-stable.

## Spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "name": "citations",
    "values": [
      {"from": "p1", "to": "p2"},
      {"from": "p1", "to": "p3"},
      {"from": "p2", "to": "p4"},
      {"from": "p3", "to": "p4"},
      {"from": "p4", "to": "p5"}
    ]
  },
  "mark": {
    "type": "network",
    "node_shape": "circle",
    "node_size": 6,
    "iterations": 200,
    "link_distance": 40,
    "seed": 42
  },
  "encoding": {
    "source": {"field": "from"},
    "target": {"field": "to"}
  }
}
```

## Knobs

| Field | Default | Effect |
|---|---|---|
| `iterations` | 200 | More iterations = smoother layout, more CPU. Capped at 2000. |
| `link_distance` | 30 | Preferred edge length (px). Higher → sparser graph. |
| `charge` | -30 | Repulsion strength. More negative → pushes nodes farther apart. |
| `seed` | 42 | Deterministic seed for initial positions. Change to reshuffle. |

## Caveats

- Multi-edge graphs render as overlapping lines today (no edge bundling).
- Disconnected components float free of each other; isolate them via
  `layer` if you need anchored panels.
- Cycles are tolerated but warn (`PRISM_WARN_NETWORK_CYCLE`) because
  the resulting layout can look messy.

## See also

- [decision_tree gallery](../gallery/tree/decision_tree.prism.json) — see the `tree` mark for strictly hierarchical data.
- [dependency_graph gallery](../gallery/network/dependency_graph.prism.json).
