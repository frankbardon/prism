# encode/format

Subset port of [d3-format](https://github.com/d3/d3-format) + a strftime-style
mini-mapping for time formats. Pure stdlib; no external date library.

## Supported number specifiers

| Spec     | Input    | Output       | Notes                                    |
| -------- | -------- | ------------ | ---------------------------------------- |
| `f`      | 1234.5   | `1234.500000` | Default 6-decimal fixed point.         |
| `.2f`    | 1234.5   | `1234.50`    | Fixed-point with 2 decimals.             |
| `,.2f`   | 1234.5   | `1,234.50`   | Thousands separator + 2 decimals.        |
| `%`      | 0.123    | `12.3%`      | Default 1-decimal percent.               |
| `.0%`    | 0.123    | `12%`        | Zero-decimal percent.                    |
| `,d`     | 1234567  | `1,234,567`  | Integer with thousands.                  |
| `d`      | 3.6      | `4`          | Integer (round-to-nearest).              |
| `.3e`    | 1234.5   | `1.235e+03`  | Scientific with 3 decimals.              |
| `.0s`    | 1234     | `1k`         | SI prefix (no decimals).                 |
| `.2s`    | 1234     | `1.23k`      | SI prefix with 2 decimals.               |
| (empty)  | 3.14159  | `3.14159`    | Default `%g` rendering.                  |

## Supported time directives

Time specs are detected by the presence of any `%X` directive other
than a bare `%` or the number-percent `N%`.

| Directive | Maps to Go layout | Renders             |
| --------- | ----------------- | ------------------- |
| `%Y`      | `2006`            | 4-digit year        |
| `%m`      | `01`              | 2-digit month       |
| `%d`      | `02`              | 2-digit day         |
| `%H`      | `15`              | 2-digit hour (24h)  |
| `%M`      | `04`              | 2-digit minute      |
| `%S`      | `05`              | 2-digit second      |
| `%L`      | `000`             | 3-digit millisecond |
| `%j`      | `002`             | 3-digit day-of-year |
| `%%`      | `%`               | literal `%`         |

## Examples

```
Parse(",.2f").Apply(1234.5)       → "1,234.50"
Parse(".0%").Apply(0.123)         → "12%"
Parse("%Y-%m-%d").Apply(t)        → "2026-05-19"
Parse("%H:%M").Apply(t)           → "14:30"
```

## Error handling

A malformed spec returns `PRISM_SPEC_011` with the spec string + parse
reason in the error context. Empty spec → no-op (default `%g`).

## Out of scope

The full d3-format grammar (fill, align, sign, symbol, width, type 'b',
type 'r', etc.) is not ported. Add a specifier branch + test when a real
fixture needs it. The 90% subset above covers every test gate in P06.
