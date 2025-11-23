# Domain Layer

The `internal/domain` package contains the core business logic and entities of the Neru application. It is pure Go code with no dependencies on external frameworks, infrastructure, or the operating system (CGo).

## Key Entities

### Element (`internal/domain/element`)
Represents a UI element on the screen.
- **ID**: Unique identifier for the element.
- **Bounds**: The rectangle area the element occupies.
- **Role**: The accessibility role (e.g., Button, Window).
- **Attributes**: Title, description, and state (clickable, etc.).

### Hint (`internal/domain/hint`)
Represents a visual hint overlay displayed on top of an element.
- **Label**: The short character sequence (e.g., "AS", "DF") used to select the hint.
- **Element**: Reference to the target UI element.
- **Position**: Where the hint should be drawn.

### Action (`internal/domain/action`)
Defines the types of actions that can be performed on elements.
- **Types**: Left Click, Right Click, Scroll, etc.

## Design Principles

- **Immutability**: Domain entities are largely immutable to prevent side effects.
- **Purity**: No side effects or external dependencies.
- **Validation**: Entities enforce their own validity constraints upon creation.
