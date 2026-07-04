---
title: Strategies
weight: 30
---

## Dynamic Strategy

The **Dynamic Strategy** displays a waiting page while your session starts.

![Demo](/assets/img/demo.gif)

{{< callout type="info" >}}
This strategy is ideal for users accessing a frontend directly, as they'll see a loading page while their services start.
{{< /callout >}}

```mermaid
sequenceDiagram
    participant User
    participant Proxy
    participant Sablier
    participant Provider
    User->>Proxy: Website Request
    Proxy->>Sablier: Reverse Proxy Plugin Request Session Status
    Sablier->>Provider: Request Instance Status
    Provider-->>Sablier: Response Instance Status
    Sablier-->>Proxy: Returns the X-Sablier-Status Header
    alt X-Sablier-Status value is not-ready
        Proxy-->>User: Serve the waiting page
        loop until X-Sablier-Status value is ready
            User->>Proxy: Self-Reload Waiting Page
            Proxy->>Sablier: Reverse Proxy Plugin Request Session Status
            Sablier->>Provider: Request Instance Status
            Provider-->>Sablier: Response Instance Status
            Sablier-->>Proxy: Returns the waiting page
            Proxy-->>User: Serve the waiting page
        end
    end
    Proxy-->>User: Content
```

The waiting page is rendered from a [theme](/strategies/themes/) — use a built-in one or provide your own.

## Blocking Strategy

The **Blocking Strategy** holds the request until your session is ready.

{{< callout type="info" >}}
This strategy is ideal for API communication, where clients expect to wait for a response.
{{< /callout >}}

```mermaid
sequenceDiagram
    participant User
    participant Proxy
    participant Sablier
    participant Provider
    User->>Proxy: Website Request
    Proxy->>Sablier: Reverse Proxy Plugin Request Session Status
    Sablier->>Provider: Request Instance Status
    alt Instance status is not-ready
        Proxy->>Sablier: Reverse Proxy Plugin Request Session Status
        Sablier->>Provider: Request Instance Status
        Provider-->>Sablier: Response Instance Status
        Sablier-->>Proxy: Returns the waiting page
    end
    Provider-->>Sablier: Response Instance Status
    Sablier-->>Proxy: Response
    Proxy-->>User: Content
```
