# Strategies

## Dynamic Strategy

The **Dynamic Strategy** provides a waiting page for your session.

![Demo](assets/img/demo.gif)

?> This strategy is well suited for a user that would access a frontend directly and expects to see a loading page.

```plantuml
@startuml

User -> Proxy: Website Request
Proxy -> Sablier: Reverse Proxy Plugin Request Session Status
Sablier -> Provider: Request Instance Status
Sablier <-- Provider: Response Instance Status
Proxy <-- Sablier: Returns the X-Sablier-Status Header

alt `X-Sablier-Status` value is `not-ready`

    User <-- Proxy: Serve the waiting page
    loop until `X-Sablier-Status` value is `ready`
        User -> Proxy: Self-Reload Waiting Page
        Proxy -> Sablier: Reverse Proxy Plugin Request Session Status
        Sablier -> Provider: Request Instance Status
        Sablier <-- Provider: Response Instance Status
        Proxy <-- Sablier: Returns the waiting page
        User <-- Proxy: Serve the waiting page
    end

end

User <-- Proxy: Content 

@enduml
```
## Blocking Strategy

The **Blocking Strategy** hangs the request until your session is ready.

?> This strategy is well suited for an API communication.

```plantuml
@startuml

User -> Proxy: Website Request
Proxy -> Sablier: Reverse Proxy Plugin Request Session Status
Sablier -> Provider: Request Instance Status

alt `Instance` status is `not-ready`
    Proxy -> Sablier: Reverse Proxy Plugin Request Session Status
    Sablier -> Provider: Request Instance Status
    Sablier <-- Provider: Response Instance Status
    Proxy <-- Sablier: Returns the waiting page
end

Sablier <-- Provider: Response Instance Status
Proxy <-- Sablier: Response 

User <-- Proxy: Content 

@enduml
```