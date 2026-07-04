# Quickstart de validación: registro global de gomemory

Prerrequisitos: binario `mem` compilado con los cambios de esta feature (`go build -o mem ./infrastructure/`), disponible en el PATH para las pruebas de resolución de proyecto.

## 1. Proyecto nuevo, sin instalación previa

```bash
mkdir -p /tmp/gomemory-quickstart/proj-a && cd /tmp/gomemory-quickstart/proj-a
git init -q
mem save -t "Prueba quickstart" -y decision "Verifica que funciona sin mem init previo"
mem search "quickstart"
```

**Esperado**: ambos comandos terminan con exit code `0`, sin pedir `mem init`. `mem search` devuelve la memoria guardada. Verificar que se creó el archivo global:

```bash
mem project   # debe reportar la ruta del mem.db dentro del store global, no .memory/mem.db
```

## 2. Aislamiento entre proyectos con el mismo nombre de carpeta

```bash
mkdir -p /tmp/gomemory-quickstart/group1/shared-name /tmp/gomemory-quickstart/group2/shared-name
cd /tmp/gomemory-quickstart/group1/shared-name && git init -q && mem save -t "Solo en group1" -y decision "A"
cd /tmp/gomemory-quickstart/group2/shared-name && git init -q && mem save -t "Solo en group2" -y decision "B"

cd /tmp/gomemory-quickstart/group1/shared-name && mem search "Solo en group2"   # debe venir vacío
cd /tmp/gomemory-quickstart/group2/shared-name && mem search "Solo en group1"   # debe venir vacío
```

**Esperado**: cero resultados cruzados — confirma SC-004 del spec.

## 3. Migración de un proyecto legado con datos

```bash
cd /home/admindocker/data/go_memory   # o cualquier repo con .memory/mem.db real con memorias
mem list | wc -l   # conteo ANTES de migrar (vía el binario viejo, o `sqlite3 .memory/mem.db "select count(*) from memories"`)
mem migrate
mem list | wc -l   # conteo DESPUÉS — debe coincidir
ls .memory/mem.db  # debe seguir existiendo (se mueve, no se copia; verificar que .memory/ quedó vacío o sin mem.db si la implementación lo remueve tras mover)
```

**Esperado**: mismo conteo antes/después, sin duplicados — confirma SC-002 del spec. Ver contrato de `mem migrate` para el caso "ambos existen".

## 4. Registro global en Claude Code

```bash
claude mcp list   # confirmar que no queda la entrada previa en conflicto (chicken_tools_sdd/ct) bajo la clave "gomemory"
mem setup-mcp --scope global --agents claude
claude mcp list   # debe mostrar "gomemory" en scope user apuntando al mem correcto
```

Abrir un proyecto nunca antes usado con gomemory y confirmar, desde el agente, que las herramientas `save_memory`/`search_memories`/etc. están disponibles sin ningún `.mcp.json` en ese repo — confirma SC-001 y SC-005 del spec.

## 5. Cero huella en el repo

```bash
cd /tmp/gomemory-quickstart/proj-a
git status --porcelain   # debe estar vacío (sin .mcp.json, sin .memory/, sin binario mem copiado)
```

**Esperado**: salida vacía — confirma SC-003 del spec.

## Referencias

- Contratos de comandos: [contracts/cli-contracts.md](./contracts/cli-contracts.md)
- Modelo de datos y normalización de migración: [data-model.md](./data-model.md)
- Decisiones y alternativas descartadas: [research.md](./research.md)
