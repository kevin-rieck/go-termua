# Target Bubble Tea and Lip Gloss directly

Bubble Toast is a toast component for Bubble Tea and Lip Gloss terminal apps, so its public package depends directly on those libraries instead of introducing a framework-agnostic core. This keeps the primary API idiomatic for Bubble Tea users and avoids adapter layers that would add complexity without serving the project's main use case.
