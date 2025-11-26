# Application Layer

The `internal/application` package implements the application's use cases and orchestrates the flow of data between the domain and infrastructure layers. It follows the Ports and Adapters (Hexagonal) architecture.

## Structure

### Ports (`internal/application/ports`)

Defines the interfaces (contracts) that the application requires to function. These interfaces are implemented by the Adapter layer.

- **AccessibilityPort**: Interface for interacting with macOS accessibility APIs.
- **OverlayPort**: Interface for drawing UI overlays (hints, grid, highlights).
- **ConfigPort**: Interface for managing application configuration.

### Services (`internal/application/services`)

Implements the business logic workflows. Services depend _only_ on domain entities and port interfaces, never on concrete adapters.

- **HintService**: Manages the lifecycle of hint generation and display.

  - Retrieves clickable elements via `AccessibilityPort`.
  - Generates hints using `hint.Generator`.
  - Displays hints via `OverlayPort`.

- **ActionService**: Handles execution of user actions.

  - Validates action requests.
  - Performs actions (click, move) via `AccessibilityPort`.
  - Draws visual feedback via `OverlayPort`.

- **ScrollService**: Manages scrolling operations.

  - Handles continuous scrolling and discrete scroll steps.
  - Draws scroll indicators.

- **GridService**: Manages the grid navigation mode.
  - Subdivides the screen into a navigable grid.
  - Handles cursor movement within the grid.
