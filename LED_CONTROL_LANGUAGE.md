# LED Control Language (LCL) Specification v2

A domain-specific language for LED pattern control, designed with a three-layer architecture optimized for LLM understanding, human readability, and microcontroller execution.

## Table of Contents

1. [Design Philosophy](#design-philosophy)
2. [Three-Layer Architecture](#three-layer-architecture)
3. [Intent Layer (LLM/Human Interface)](#intent-layer-llmhuman-interface)
4. [Specification Layer (Precise Control)](#specification-layer-precise-control)
5. [Execution Layer (Bytecode)](#execution-layer-bytecode)
6. [Semantic Value System](#semantic-value-system)
7. [Effect Reference](#effect-reference)
8. [Color System](#color-system)
9. [Timing and Animation](#timing-and-animation)
10. [Spatial Mapping](#spatial-mapping)
11. [LLM Integration Guidelines](#llm-integration-guidelines)
12. [Implementation Guidelines](#implementation-guidelines)
13. [Examples](#examples)
14. [References](#references)

---

## Design Philosophy

### Core Principles

1. **Intent-First Design**: Describe *what* you want, not *how* to achieve it
2. **Semantic Parameters**: Named values (`tall`, `bright`) instead of magic numbers (`55`, `200`)
3. **LLM-Optimized**: Natural language alignment, self-documenting constraints
4. **Graceful Degradation**: Three layers allow different precision levels
5. **Forgiving Input**: Synonyms accepted, flexible formatting
6. **Self-Describing**: Schema queryable inline

### Why Three Layers?

| Layer | Optimized For | Format | Example |
|-------|---------------|--------|---------|
| **Intent** | LLM + Human | YAML-like, semantic | `flame_height: tall` |
| **Specification** | Debugging + Override | JSON-like, precise | `cooling: 55` |
| **Execution** | Microcontroller | Binary bytecode | `0x51 0x37` |

### Inspiration Sources

| Source | Contribution |
|--------|--------------|
| [WLED JSON API](https://kno.wled.ge/interfaces/json-api/) | Segment-based addressing, effect metadata |
| [FastLED](https://fastled.io/) | Color palettes, HSV operations, gradient definitions |
| [DMX512](https://en.wikipedia.org/wiki/DMX512) | Channel-based addressing, universe concept |
| [Lizard DSL](https://github.com/zauberzeug/lizard) | Hardware behavior specification for MCUs |
| Natural Language Processing | Semantic slot filling, intent recognition |

---

## Three-Layer Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         USER / LLM                                  │
│                                                                     │
│   "Create a cozy campfire effect with tall orange flames"           │
│                              │                                      │
│                              ▼                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    INTENT LAYER (v2)                          │  │
│  │                                                               │  │
│  │  effect: fire                                                 │  │
│  │  name: "Cozy Campfire"                                        │  │
│  │  behavior:                                                    │  │
│  │    flame_height: tall                                         │  │
│  │    spark_frequency: frequent                                  │  │
│  │  appearance:                                                  │  │
│  │    color_scheme: warm_orange                                  │  │
│  │    brightness: bright                                         │  │
│  │                                                               │  │
│  │  [Semantic, forgiving, LLM-friendly]                          │  │
│  └───────────────────────────┬───────────────────────────────────┘  │
└──────────────────────────────┼──────────────────────────────────────┘
                               │ Compiler (semantic → numeric)
                               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                 SPECIFICATION LAYER (v1)                      │   │
│  │                                                               │   │
│  │  pattern "Cozy Campfire" {                                    │   │
│  │    type: fire                                                 │   │
│  │    cooling: 55                                                │   │
│  │    sparking: 120                                              │   │
│  │    palette: [#000000, #331100, #FF4400, #FFAA00, #FFFFFF]     │   │
│  │  }                                                            │   │
│  │                                                               │   │
│  │  [Precise, debuggable, overridable]                           │   │
│  └───────────────────────────┬───────────────────────────────────┘   │
│                              │ Compiler (text → binary)              │
│                              ▼                                       │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                   EXECUTION LAYER                             │   │
│  │                                                               │   │
│  │  4C 43 4C 02 00 1A B7 00 50 07 51 37 00 51 78 01 ...          │   │
│  │                                                               │   │
│  │  [Compact bytecode for MCU]                                   │   │
│  └───────────────────────────────────────────────────────────────┘   │
│                           MICROCONTROLLER                            │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Intent Layer (LLM/Human Interface)

The Intent Layer is the **primary interface** for LLMs and humans. It uses semantic values, accepts synonyms, and self-documents constraints.

### Basic Structure

```yaml
effect: <effect_type>
name: "<human readable name>"           # optional

behavior:
  <semantic_parameter>: <semantic_value>

appearance:
  <semantic_parameter>: <semantic_value>

timing:
  <semantic_parameter>: <semantic_value>

spatial:
  <semantic_parameter>: <semantic_value>
```

### Complete Fire Effect Example

```yaml
effect: fire
name: "Cozy Campfire"

# Behavioral parameters (how the effect acts)
behavior:
  flame_height: tall            # short | medium | tall | very_tall
  spark_frequency: frequent     # rare | occasional | frequent | high | intense
  flame_speed: medium           # slow | medium | fast
  turbulence: moderate          # calm | moderate | chaotic

# Visual parameters (how the effect looks)
appearance:
  color_scheme: classic_fire    # classic_fire | blue_gas | green_toxic | purple_mystical | custom
  brightness: bright            # dim | medium | bright | full
  saturation: vivid             # muted | medium | vivid | full

# Timing parameters (when things happen)
timing:
  speed: medium                 # frozen | very_slow | slow | medium | fast | very_fast

# Spatial parameters (where things happen)
spatial:
  direction: up                 # up | down | left | right | outward | inward
  coverage: full                # quarter | third | half | two_thirds | full
  origin: bottom                # top | bottom | left | right | center
```

### Flexible Input Acceptance

The Intent Layer accepts multiple equivalent inputs:

```yaml
# All of these are equivalent:
flame_height: tall
flame_height: TALL
flame_height: Tall
flame_height: "tall"
flame_height: high              # synonym accepted
flame_height: long              # synonym accepted

# Color flexibility:
color: red
color: RED
color: "#FF0000"
color: "rgb(255, 0, 0)"
color: "warm red"               # descriptive accepted
color: crimson                  # named color

# Speed flexibility:
speed: fast
speed: quick                    # synonym
speed: rapid                    # synonym
speed: 0.8                      # normalized 0-1 also works
```

### Natural Language Input (Experimental)

```yaml
# Natural language descriptions can be parsed:
effect: "gentle orange flames rising slowly, like a calm campfire"

# Parser extracts:
#   effect_type: fire
#   color_scheme: warm_orange
#   flame_speed: slow
#   spark_frequency: occasional (inferred from "gentle")
#   flame_height: medium (default)
```

### Schema Queries

LLMs can query the language schema inline:

```yaml
# Query available effects:
?effects
# Returns: solid, gradient, wave, chase, sparkle, breathe, fire, plasma, ripple, radar, honeycomb, sequence, reactive

# Query effect parameters:
?fire
# Returns:
# fire: Realistic flame simulation
#   behavior:
#     flame_height: short | medium | tall | very_tall (default: medium)
#     spark_frequency: rare | occasional | frequent | high | intense (default: frequent)
#     flame_speed: slow | medium | fast (default: medium)
#     turbulence: calm | moderate | chaotic (default: moderate)
#   appearance:
#     color_scheme: classic_fire | blue_gas | green_toxic | purple_mystical | custom
#     brightness: dim | medium | bright | full (default: bright)

# Query specific parameter:
?fire.flame_height
# Returns:
# flame_height: How high flames reach on the LED strip
#   Values: short (25%) | medium (50%) | tall (75%) | very_tall (100%)
#   Default: medium
#   Maps to: cooling parameter (200 | 100 | 55 | 20)
```

---

## Specification Layer (Precise Control)

The Specification Layer provides **exact numeric control** when needed. This is what the Intent Layer compiles to.

### Syntax

```lcl
pattern "<name>" {
  type: <effect_type>
  <parameter>: <numeric_value>
  ...
}
```

### Fire Effect (Specification Layer)

```lcl
pattern "Cozy Campfire" {
  type: fire
  cooling: 55           # 0-255: higher = shorter flames
  sparking: 120         # 0-255: higher = more sparks

  palette: [
    { pos: 0.0, color: #000000 },
    { pos: 0.25, color: #331100 },
    { pos: 0.5, color: #FF4400 },
    { pos: 0.75, color: #FFAA00 },
    { pos: 1.0, color: #FFFFFF }
  ]

  direction: up
  height: 1.0
}
```

### When to Use Specification Layer

- **Debugging**: See exact values being used
- **Fine-tuning**: Adjust specific numeric parameters
- **Override**: Bypass semantic mapping for precise control
- **Legacy**: Import from existing systems

---

## Execution Layer (Bytecode)

The Execution Layer is **compact binary bytecode** for microcontroller execution.

### Header Format

```
┌───────────┬───────────┬───────────┬───────────┐
│  'L'      │  'C'      │  'L'      │  Version  │
│  0x4C     │  0x43     │  0x4C     │  0x02     │
├───────────┴───────────┼───────────┴───────────┤
│  Length (16-bit)      │  Checksum │  Flags    │
└───────────────────────┴───────────┴───────────┘
```

### Instruction Format

```
┌─────────┬─────────┬─────────────────────┐
│ Opcode  │ Flags   │ Operands            │
│ (8 bit) │ (8 bit) │ (variable length)   │
└─────────┴─────────┴─────────────────────┘
```

See [Appendix A](#appendix-a-complete-opcode-table) for complete opcode reference.

---

## Semantic Value System

The core innovation of LCL v2 is the **semantic value system** that maps human-understandable terms to precise numeric values.

### Semantic Value Tables

#### Speed Values

| Semantic | Numeric | Multiplier | Description |
|----------|---------|------------|-------------|
| `frozen` | 0 | 0.0 | No movement |
| `very_slow` | 1 | 0.1 | Barely perceptible |
| `slow` | 2 | 0.3 | Gentle, relaxed |
| `medium` | 3 | 0.5 | Natural pace |
| `fast` | 4 | 0.8 | Energetic |
| `very_fast` | 5 | 1.0 | Rapid, intense |

**Synonyms**: `quick`→`fast`, `rapid`→`fast`, `sluggish`→`slow`, `crawling`→`very_slow`

#### Brightness Values

| Semantic | Numeric | Percentage | Use Case |
|----------|---------|------------|----------|
| `dim` | 1 | 20% | Ambient, night mode |
| `medium` | 2 | 50% | Normal indoor |
| `bright` | 3 | 80% | Well-lit, attention |
| `full` | 4 | 100% | Maximum output |

**Synonyms**: `low`→`dim`, `high`→`bright`, `max`→`full`

#### Fire-Specific: Flame Height

| Semantic | Cooling Value | Visual Result |
|----------|---------------|---------------|
| `very_short` | 200 | ~10% of strip, embers |
| `short` | 150 | ~25% of strip, low flames |
| `medium` | 100 | ~50% of strip, normal fire |
| `tall` | 55 | ~75% of strip, roaring fire |
| `very_tall` | 20 | ~100% of strip, inferno |

**Synonyms**: `low`→`short`, `high`→`tall`, `small`→`short`, `large`→`tall`

#### Fire-Specific: Spark Frequency

| Semantic | Sparking Value | Visual Result |
|----------|----------------|---------------|
| `rare` | 30 | Dying embers |
| `occasional` | 60 | Calm, steady |
| `frequent` | 120 | Normal campfire |
| `high` | 180 | Active, lively |
| `intense` | 230 | Roaring, explosive |

**Synonyms**: `few`→`rare`, `some`→`occasional`, `many`→`high`, `lots`→`intense`

### Color Scheme Mappings

| Scheme Name | Palette Colors | Use Case |
|-------------|----------------|----------|
| `classic_fire` | black → dark_red → orange → yellow → white | Traditional fire |
| `warm_orange` | black → brown → orange → gold → white | Warm, cozy |
| `blue_gas` | black → dark_blue → blue → cyan → white | Gas flame, sci-fi |
| `green_toxic` | black → dark_green → green → lime → white | Toxic, magical |
| `purple_mystical` | black → purple → magenta → pink → white | Fantasy, mystical |
| `white_hot` | black → red → orange → yellow → white | Industrial, intense |

---

## Effect Reference

### Effect: `solid`

Fill all LEDs with a single color.

**Intent Layer:**
```yaml
effect: solid
appearance:
  color: warm_white
  brightness: medium
```

**Specification Layer:**
```lcl
pattern "Solid" { type: solid; color: #FFE4B5; brightness: 0.5 }
```

---

### Effect: `gradient`

Smooth color transition across LEDs.

**Intent Layer:**
```yaml
effect: gradient
appearance:
  color_scheme: sunset        # sunset | ocean | forest | rainbow | custom
  brightness: bright
behavior:
  smoothness: high            # low | medium | high
timing:
  speed: slow                 # how fast it animates (if animated)
  animation: shifting         # none | shifting | bouncing
```

**Parameters:**

| Parameter | Values | Default | Description |
|-----------|--------|---------|-------------|
| `color_scheme` | sunset, ocean, forest, rainbow, custom | rainbow | Color palette |
| `smoothness` | low, medium, high | high | Interpolation quality |
| `animation` | none, shifting, bouncing | none | Movement style |

---

### Effect: `wave`

Sinusoidal color variation.

**Intent Layer:**
```yaml
effect: wave
appearance:
  color_scheme: ocean
  brightness: bright
behavior:
  wave_count: few             # one | few | several | many
  wave_shape: smooth          # smooth | sharp | choppy
timing:
  speed: medium
spatial:
  direction: forward          # forward | backward | bounce
```

**Parameters:**

| Parameter | Values | Maps To |
|-----------|--------|---------|
| `wave_count` | one(1), few(2), several(4), many(8) | frequency |
| `wave_shape` | smooth(sine), sharp(triangle), choppy(noise) | waveform |

---

### Effect: `chase`

Moving segment of illuminated LEDs.

**Intent Layer:**
```yaml
effect: chase
appearance:
  color: cyan
  background: black
  brightness: full
behavior:
  head_size: small            # tiny | small | medium | large
  tail_length: long           # none | short | medium | long | very_long
  tail_style: fade            # sharp | fade | sparkle
  count: single               # single | double | triple | many
timing:
  speed: fast
spatial:
  direction: forward          # forward | backward | bounce
```

**Parameters:**

| Parameter | Values | Maps To |
|-----------|--------|---------|
| `head_size` | tiny(1), small(3), medium(5), large(10) | head_size LEDs |
| `tail_length` | none(0), short(5), medium(10), long(20), very_long(40) | tail_size LEDs |
| `tail_style` | sharp(linear), fade(exponential), sparkle(random) | tail_decay |

---

### Effect: `sparkle`

Random twinkling points of light.

**Intent Layer:**
```yaml
effect: sparkle
appearance:
  color: white
  color_variation: some       # none | slight | some | lots
  background: dark_blue
  brightness: full
behavior:
  density: medium             # sparse | light | medium | dense | packed
  lifetime: medium            # flash | short | medium | long
timing:
  fade_in: quick              # instant | quick | gradual
  fade_out: gradual           # instant | quick | gradual
```

**Parameters:**

| Parameter | Values | Maps To |
|-----------|--------|---------|
| `density` | sparse(0.02), light(0.05), medium(0.1), dense(0.2), packed(0.4) | probability |
| `lifetime` | flash(100ms), short(250ms), medium(500ms), long(1000ms) | duration |

---

### Effect: `breathe`

Pulsing brightness effect.

**Intent Layer:**
```yaml
effect: breathe
appearance:
  color: red
  brightness_range: wide      # subtle | moderate | wide | full
behavior:
  rhythm: calm                # calm | relaxed | steady | energetic | frantic
  pattern: smooth             # smooth | heartbeat | double_pulse
```

**Parameters:**

| Parameter | Values | Maps To |
|-----------|--------|---------|
| `rhythm` | calm(4s), relaxed(3s), steady(2s), energetic(1s), frantic(0.5s) | period |
| `brightness_range` | subtle(0.7-1.0), moderate(0.4-1.0), wide(0.1-1.0), full(0.0-1.0) | min/max |
| `pattern` | smooth(sine), heartbeat(cardiac), double_pulse(custom) | waveform |

---

### Effect: `fire`

Realistic flame simulation.

**Intent Layer:**
```yaml
effect: fire
name: "Campfire"
behavior:
  flame_height: tall          # very_short | short | medium | tall | very_tall
  spark_frequency: frequent   # rare | occasional | frequent | high | intense
  flame_speed: medium         # slow | medium | fast
  turbulence: moderate        # calm | moderate | chaotic
appearance:
  color_scheme: classic_fire  # classic_fire | blue_gas | green_toxic | purple_mystical
  brightness: bright
spatial:
  direction: up               # up | down | left | right
```

**Algorithm (Fire2012-based):**
```
each frame:
    # 1. Cool down every cell
    for each LED:
        heat[i] -= random(0, cooling_factor)

    # 2. Heat diffuses upward
    for i from top to bottom:
        heat[i] = average(heat[i-1], heat[i-2])

    # 3. Randomly ignite sparks at bottom
    if random() < spark_probability:
        heat[random(0..7)] += random(160, 255)

    # 4. Map heat to color via palette
    for each LED:
        color = palette.sample(heat[i] / 255)
```

---

### Effect: `plasma`

2D psychedelic plasma effect.

**Intent Layer:**
```yaml
effect: plasma
appearance:
  color_scheme: rainbow
  brightness: bright
behavior:
  complexity: medium          # simple | medium | complex | chaotic
  scale: medium               # fine | medium | coarse
timing:
  speed: slow
```

---

### Effect: `ripple`

Expanding circular waves from a point.

**Intent Layer:**
```yaml
effect: ripple
appearance:
  color: cyan
  background: black
behavior:
  wave_count: few             # one | few | several | many
  decay: gradual              # none | gradual | rapid
timing:
  speed: medium
spatial:
  origin: center              # center | random | click_position
```

---

### Effect: `sequence`

Multiple effects played in order.

**Intent Layer:**
```yaml
effect: sequence
behavior:
  loop: yes                   # yes | no | count(3)
  transition: fade            # cut | fade | wipe | dissolve
  transition_time: quick      # instant | quick | medium | slow

steps:
  - effect: solid
    appearance: { color: red }
    duration: short           # very_short | short | medium | long | very_long

  - effect: fire
    behavior: { flame_height: tall }
    duration: long

  - effect: sparkle
    appearance: { color: white }
    duration: medium
```

**Duration mappings**: very_short(1s), short(2s), medium(5s), long(10s), very_long(30s)

---

## Color System

### Color Input Formats

```yaml
# Named colors (CSS color names)
color: red
color: coral
color: darkslateblue

# Hex RGB
color: "#FF5500"
color: "#F50"              # shorthand

# Hex RGBA
color: "#FF550080"

# RGB function
color: rgb(255, 85, 0)

# RGBA function
color: rgba(255, 85, 0, 0.5)

# HSV function (hue: 0-360, sat: 0-100, val: 0-100)
color: hsv(20, 100, 100)

# HSL function
color: hsl(20, 100, 50)

# Temperature (Kelvin)
color: warm_white          # 2700K
color: cool_white          # 4000K
color: daylight            # 6500K
color: kelvin(3200)        # custom

# Descriptive (parsed)
color: "bright orange"
color: "soft pink"
color: "deep blue"
```

### Built-in Color Schemes

| Name | Description | Colors |
|------|-------------|--------|
| `rainbow` | Full spectrum | Red → Orange → Yellow → Green → Cyan → Blue → Purple → Red |
| `sunset` | Warm evening | Orange → Pink → Purple → Dark Blue |
| `ocean` | Cool water | Dark Blue → Blue → Cyan → Teal |
| `forest` | Natural greens | Dark Green → Green → Lime → Yellow |
| `lava` | Hot magma | Black → Red → Orange → Yellow |
| `aurora` | Northern lights | Green → Cyan → Purple → Pink |
| `fire` | Flame colors | Black → Red → Orange → Yellow → White |
| `ice` | Cold winter | White → Cyan → Blue → Dark Blue |
| `party` | Vibrant mix | Magenta → Cyan → Yellow → Magenta |

### Custom Color Schemes

```yaml
appearance:
  color_scheme: custom
  custom_colors:
    - position: 0%
      color: black
    - position: 25%
      color: dark_red
    - position: 50%
      color: orange
    - position: 75%
      color: yellow
    - position: 100%
      color: white
```

---

## Timing and Animation

### Speed Scale

| Semantic | Period | BPM Equivalent | Use Case |
|----------|--------|----------------|----------|
| `frozen` | ∞ | 0 | Static display |
| `glacial` | 30s | 2 | Ambient, meditative |
| `very_slow` | 10s | 6 | Relaxing, background |
| `slow` | 4s | 15 | Calm, gentle |
| `medium` | 2s | 30 | Natural, balanced |
| `fast` | 1s | 60 | Energetic, active |
| `very_fast` | 0.5s | 120 | Intense, exciting |
| `frantic` | 0.25s | 240 | Strobe-like |

### Duration Scale

| Semantic | Time | Use Case |
|----------|------|----------|
| `instant` | 0ms | Immediate |
| `flash` | 50ms | Quick flash |
| `quick` | 200ms | Fast transition |
| `short` | 500ms | Brief |
| `medium` | 1000ms | Standard |
| `long` | 2000ms | Leisurely |
| `very_long` | 5000ms | Extended |

### Easing Functions

```yaml
timing:
  easing: smooth              # linear | smooth | bounce | elastic | snap

# Detailed mappings:
# linear     → linear
# smooth     → ease_in_out
# ease_in    → ease_in_quad
# ease_out   → ease_out_quad
# bounce     → ease_out_bounce
# elastic    → ease_out_elastic
# snap       → ease_in_expo
```

---

## Spatial Mapping

### 1D Strip Configuration

```yaml
spatial:
  layout: strip
  led_count: 60
  direction: forward          # forward | reverse
```

### 2D Grid Configuration

```yaml
spatial:
  layout: grid
  width: 16
  height: 16
  origin: top_left            # top_left | top_right | bottom_left | bottom_right | center
  wiring: serpentine          # sequential | serpentine
```

### Radial Configuration

```yaml
spatial:
  layout: radial
  rings: 5
  leds_per_ring: [8, 16, 24, 32, 40]
  origin: center
```

---

## LLM Integration Guidelines

### Prompt Engineering for LCL

When asking an LLM to generate LCL:

**Good Prompt:**
> "Create an LED effect that looks like a cozy fireplace - warm orange flames that dance gently"

**LLM Should Generate:**
```yaml
effect: fire
name: "Cozy Fireplace"
behavior:
  flame_height: medium
  spark_frequency: occasional
  flame_speed: slow
  turbulence: calm
appearance:
  color_scheme: warm_orange
  brightness: medium
```

### Error Handling

When LLM generates invalid values:

```yaml
# LLM writes:
flame_height: huge           # invalid value

# Parser response:
# WARNING: Unknown value "huge" for flame_height
# Valid values: very_short | short | medium | tall | very_tall
# Suggestion: Did you mean "very_tall"?
# Using default: medium
```

### Validation Rules

1. **Unknown effect**: Suggest similar effect names
2. **Unknown parameter**: List valid parameters for effect
3. **Unknown value**: Show valid values with descriptions
4. **Type mismatch**: Attempt coercion, warn if ambiguous
5. **Out of range**: Clamp to valid range, warn

### LLM Response Format

When explaining generated LCL, structure as:

```
## Effect: [Name]

**Type**: [effect type]
**Mood**: [description of visual feel]

### Key Settings:
- **[Parameter]**: [value] - [why this was chosen]

### Visual Result:
[Description of what user will see]

### Adjustments:
- To make it [more X], change [parameter] to [value]
- To make it [more Y], change [parameter] to [value]
```

---

## Implementation Guidelines

### Parser Requirements

```
┌─────────────────────────────────────────────────────────┐
│                    INTENT PARSER                        │
├─────────────────────────────────────────────────────────┤
│  1. Tokenize YAML-like input                            │
│  2. Normalize case (case-insensitive)                   │
│  3. Resolve synonyms → canonical names                  │
│  4. Validate against schema                             │
│  5. Map semantic values → numeric values                │
│  6. Generate Specification Layer output                 │
└─────────────────────────────────────────────────────────┘
```

### Synonym Resolution Table

```javascript
const synonyms = {
  // Speed
  "quick": "fast",
  "rapid": "fast",
  "swift": "fast",
  "sluggish": "slow",
  "crawling": "very_slow",

  // Brightness
  "low": "dim",
  "high": "bright",
  "max": "full",
  "maximum": "full",

  // Size
  "small": "short",
  "large": "tall",
  "big": "tall",
  "tiny": "very_short",
  "huge": "very_tall",

  // Frequency
  "few": "rare",
  "some": "occasional",
  "many": "high",
  "lots": "intense",

  // Boolean
  "yes": true,
  "no": false,
  "on": true,
  "off": false,
  "enabled": true,
  "disabled": false
};
```

### Microcontroller Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| Flash | 16 KB | 64 KB |
| RAM | 2 KB | 8 KB |
| Clock | 16 MHz | 80+ MHz |
| Stack | 128 bytes | 256 bytes |

### Memory Layout

```
┌─────────────────────────────────────┐
│ Bytecode Buffer (ROM/Flash)         │
│ Size: Variable (pattern dependent)  │
├─────────────────────────────────────┤
│ VM Registers & Stack                │
│ Size: 128-256 bytes                 │
├─────────────────────────────────────┤
│ Pattern State                       │
│ Size: 64-128 bytes                  │
├─────────────────────────────────────┤
│ LED Buffer (RGBW)                   │
│ Size: led_count * 4 bytes           │
├─────────────────────────────────────┤
│ Palette Cache                       │
│ Size: 16 * 3 = 48 bytes             │
└─────────────────────────────────────┘
```

---

## Examples

### Example 1: Cozy Evening Ambiance

**Natural Request:**
> "I want warm, gentle lighting for a relaxed evening"

**Intent Layer:**
```yaml
effect: breathe
name: "Evening Ambiance"
appearance:
  color: warm_white
  brightness: medium
behavior:
  rhythm: calm
  brightness_range: subtle
```

**Compiles to Specification Layer:**
```lcl
pattern "Evening Ambiance" {
  type: breathe
  color: #FFE4B5
  min_brightness: 0.7
  max_brightness: 1.0
  period: 4000
  waveform: sine
}
```

---

### Example 2: Energetic Party Mode

**Natural Request:**
> "Make it exciting for a party - lots of movement and color!"

**Intent Layer:**
```yaml
effect: sequence
name: "Party Mode"
behavior:
  loop: yes
  transition: wipe
  transition_time: quick

steps:
  - effect: chase
    appearance:
      color_scheme: party
    behavior:
      head_size: medium
      tail_length: long
      count: triple
    timing:
      speed: fast
    duration: medium

  - effect: sparkle
    appearance:
      color: white
      background: purple
    behavior:
      density: dense
    duration: short

  - effect: wave
    appearance:
      color_scheme: rainbow
    behavior:
      wave_count: several
    timing:
      speed: fast
    duration: medium
```

---

### Example 3: Campfire with Variations

**Base Effect:**
```yaml
effect: fire
name: "Campfire"
behavior:
  flame_height: tall
  spark_frequency: frequent
appearance:
  color_scheme: classic_fire
```

**Variation - Dying Embers:**
```yaml
base: campfire
modify:
  behavior:
    flame_height: very_short
    spark_frequency: rare
  appearance:
    brightness: dim
```

**Variation - Roaring Blaze:**
```yaml
base: campfire
modify:
  behavior:
    flame_height: very_tall
    spark_frequency: intense
    turbulence: chaotic
  appearance:
    brightness: full
```

**Variation - Blue Gas Flame:**
```yaml
base: campfire
modify:
  appearance:
    color_scheme: blue_gas
  behavior:
    flame_height: short
    turbulence: calm
```

---

### Example 4: Interactive Effect

```yaml
effect: reactive
name: "Sound Reactive Fire"

base_effect:
  effect: fire
  behavior:
    flame_height: medium
    spark_frequency: occasional

input: audio_level

reactions:
  - trigger: level_above
    threshold: low
    action: increase
    target: spark_frequency
    amount: moderate

  - trigger: level_above
    threshold: high
    action: flash
    color: white
    duration: flash

  - trigger: beat_detected
    action: boost
    target: flame_height
    duration: quick
```

---

## References

### Protocols and Standards

- [DMX512 Wikipedia](https://en.wikipedia.org/wiki/DMX512) - Digital lighting control standard
- [Art-Net Specification](https://art-net.org.uk/downloads/art-net.pdf) - DMX over Ethernet protocol
- [E1.31 sACN](https://wiki.openlighting.org/index.php/E1.31) - Streaming ACN protocol

### LED Libraries and Frameworks

- [FastLED Library](https://fastled.io/) - Arduino LED library with palettes and math
- [WLED Project](https://kno.wled.ge/) - ESP-based LED controller with JSON API
- [led-control](https://github.com/jackw01/led-control) - Python-based LED animation system
- [quickPatterns](https://github.com/brimshot/quickPatterns) - FastLED pattern engine

### DSL Design Resources

- [Lizard DSL](https://github.com/zauberzeug/lizard) - Hardware behavior DSL for microcontrollers
- [DSL Design Patterns](https://www2.dmst.aueb.gr/dds/pubs/jrnl/2000-JSS-DSLPatterns/html/dslpat.html) - Academic patterns for DSL design

### Implementation References

- [Crafting Interpreters - Bytecode VM](https://craftinginterpreters.com/a-bytecode-virtual-machine.html) - VM implementation guide
- [Tiny Interpreters for Microcontrollers](https://dercuano.github.io/notes/tiny-interpreters-for-microcontrollers.html) - Resource-constrained VMs
- [Easing Functions](https://easings.net/) - Animation timing functions
- [Color Interpolation Secrets](https://www.alanzucconi.com/2016/01/06/colour-interpolation/) - HSV vs RGB interpolation

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2024-12 | Initial specification (programmer-focused) |
| 2.0 | 2024-12 | Intent-First redesign for LLM compatibility |

---

## Appendix A: Complete Opcode Table

| Opcode | Mnemonic | Stack Effect | Description |
|--------|----------|--------------|-------------|
| `0x00` | NOP | - | No operation |
| `0x01` | PUSH_U8 | -> a | Push unsigned byte |
| `0x02` | PUSH_U16 | -> a | Push unsigned word |
| `0x03` | PUSH_I16 | -> a | Push signed word |
| `0x04` | PUSH_F16 | -> a | Push fixed-point |
| `0x05` | PUSH_COLOR | -> c | Push RGB color |
| `0x06` | POP | a -> | Discard TOS |
| `0x07` | DUP | a -> a a | Duplicate TOS |
| `0x08` | SWAP | a b -> b a | Swap top two |
| `0x10` | ADD | a b -> (a+b) | Addition |
| `0x11` | SUB | a b -> (a-b) | Subtraction |
| `0x12` | MUL | a b -> (a*b) | Multiplication |
| `0x13` | DIV | a b -> (a/b) | Division |
| `0x14` | MOD | a b -> (a%b) | Modulo |
| `0x15` | NEG | a -> (-a) | Negate |
| `0x20` | EQ | a b -> (a==b) | Equal |
| `0x21` | NE | a b -> (a!=b) | Not equal |
| `0x22` | LT | a b -> (a<b) | Less than |
| `0x23` | GT | a b -> (a>b) | Greater than |
| `0x24` | LE | a b -> (a<=b) | Less or equal |
| `0x25` | GE | a b -> (a>=b) | Greater or equal |
| `0x30` | SIN | a -> sin(a) | Sine |
| `0x31` | COS | a -> cos(a) | Cosine |
| `0x32` | TAN | a -> tan(a) | Tangent |
| `0x33` | SQRT | a -> sqrt(a) | Square root |
| `0x34` | ABS | a -> abs(a) | Absolute value |
| `0x35` | MIN | a b -> min(a,b) | Minimum |
| `0x36` | MAX | a b -> max(a,b) | Maximum |
| `0x37` | CLAMP | a lo hi -> clamp | Clamp to range |
| `0x38` | LERP | a b t -> lerp | Linear interpolate |
| `0x39` | MAP | v i1 i2 o1 o2 -> m | Range mapping |
| `0x3A` | NOISE | x -> noise(x) | Perlin noise |
| `0x40` | RGB | r g b -> color | Create RGB color |
| `0x41` | HSV | h s v -> color | Create HSV color |
| `0x42` | BLEND | c1 c2 t -> color | Blend colors |
| `0x43` | BRIGHTNESS | c b -> color | Adjust brightness |
| `0x44` | PALETTE | idx pal -> color | Sample palette |
| `0x45` | HUE_SHIFT | c h -> color | Shift hue |
| `0x50` | SET_PATTERN | type -> | Set pattern type |
| `0x51` | SET_PARAM | val param -> | Set parameter |
| `0x52` | SET_MAPPING | type -> | Set mapping type |
| `0x53` | BEGIN_FRAME | - | Start frame |
| `0x54` | END_FRAME | - | Output frame |
| `0x55` | SET_LED | color idx -> | Set LED color |
| `0x56` | FILL | color start end -> | Fill range |
| `0x60` | JUMP | addr -> | Jump to address |
| `0x61` | JUMP_IF | cond addr -> | Conditional jump |
| `0x62` | JUMP_UNLESS | cond addr -> | Inverse conditional |
| `0x63` | CALL | addr -> | Call subroutine |
| `0x64` | RETURN | - | Return |
| `0x65` | FOR_EACH_LED | - | Begin LED loop |
| `0x66` | NEXT_LED | - | Continue loop |
| `0x67` | HALT | - | Stop execution |
| `0x70` | LOAD_VAR | idx -> val | Load variable |
| `0x71` | STORE_VAR | val idx -> | Store variable |
| `0x72` | GET_TIME | -> time | Push time |
| `0x73` | GET_FRAME | -> frame | Push frame count |
| `0x74` | GET_LED_COUNT | -> count | Push LED count |
| `0x75` | GET_LED_INDEX | -> idx | Push current index |
| `0x76` | GET_X | -> x | Push X coordinate |
| `0x77` | GET_Y | -> y | Push Y coordinate |

---

## Appendix B: Effect Type IDs

| ID | Effect Type | Category |
|----|-------------|----------|
| `0x01` | solid | Basic |
| `0x02` | gradient | Basic |
| `0x03` | wave | Animated |
| `0x04` | chase | Animated |
| `0x05` | sparkle | Animated |
| `0x06` | breathe | Animated |
| `0x07` | fire | Simulation |
| `0x08` | plasma | 2D |
| `0x09` | ripple | 2D |
| `0x0A` | radar | Radial |
| `0x0B` | honeycomb | Hexagonal |
| `0x0C` | matrix | 2D Animated |
| `0x0D` | noise | Procedural |
| `0x0E` | strobe | Flash |
| `0x0F` | rainbow | Classic |
| `0x10` | twinkle | Animated |
| `0x11` | meteor | Animated |
| `0x12` | scan | Animated |
| `0x13` | larson | Animated |
| `0x14` | theater | Animated |
| `0x20` | sequence | Meta |
| `0x21` | reactive | Meta |
| `0x22` | layered | Meta |
| `0x80+` | User Defined | Custom |

---

## Appendix C: Semantic Value Quick Reference

### Universal Values

| Category | Values |
|----------|--------|
| **Speed** | frozen, glacial, very_slow, slow, medium, fast, very_fast, frantic |
| **Brightness** | dim, medium, bright, full |
| **Duration** | instant, flash, quick, short, medium, long, very_long |
| **Direction** | up, down, left, right, forward, backward, inward, outward, clockwise, counterclockwise |
| **Size** | tiny, small, medium, large, huge |
| **Density** | sparse, light, medium, dense, packed |
| **Intensity** | subtle, moderate, strong, intense |

### Effect-Specific Values

| Effect | Parameter | Values |
|--------|-----------|--------|
| **fire** | flame_height | very_short, short, medium, tall, very_tall |
| **fire** | spark_frequency | rare, occasional, frequent, high, intense |
| **fire** | turbulence | calm, moderate, chaotic |
| **wave** | wave_count | one, few, several, many |
| **wave** | wave_shape | smooth, sharp, choppy |
| **chase** | tail_style | sharp, fade, sparkle |
| **breathe** | rhythm | calm, relaxed, steady, energetic, frantic |
| **breathe** | pattern | smooth, heartbeat, double_pulse |
