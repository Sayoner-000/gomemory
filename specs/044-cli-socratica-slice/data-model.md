# Data Model: Entradas CLI y MCP de Socrática como slice

**Feature**: 044-cli-socratica-slice

Esta refactorización no introduce entidades, persistencia ni cambios de esquema. Se conservan `domain.RefinementFlow`, los contratos de `usecases.SocraticaFlow` y el DTO de entrada/salida de la herramienta MCP.

## Elementos de integración

| Elemento | Responsabilidad | Compatibilidad |
|---|---|---|
| `clikit.Deps` | Dependencias inyectadas a entradas primarias | Campos y alias existentes permanecen disponibles |
| `clikit.ReadBounded` | Lectura completa de un stream hasta un máximo | Retiene sentinel y error por exceder límite |
| selector de harness en clikit | Presentación y elección desde catálogo | Conserva opciones, exclusiones y errores |
| `socratica.Run` | Ejecutar una invocación CLI | Devuelve códigos actuales; no termina proceso |
| `socratica.RegisterTools` | Añadir la herramienta de flujo Socrática al servidor MCP | Registra `socratica_flow` una vez |
