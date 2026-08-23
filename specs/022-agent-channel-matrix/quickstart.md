# Quickstart — validación de la matriz de canales

Guía de validación end-to-end. No contiene implementación: comprueba que la feature funciona
contra el binario real, siguiendo la regla del proyecto de que «verde en tests» no es «funciona».

## Prerrequisitos

- Go 1.24 y el repositorio en la raíz de trabajo.
- Nada más: la verificación no requiere red ni entorno especial.

## 1. La batería, incluida la verificación de la matriz

```bash
go build ./... && go test ./...
```

**Esperado**: todo en verde, incluido `tests/contract/channel_matrix_test.go`.

## 2. El contrato falla ante una celda sin declarar

Añade temporalmente a la matriz una celda sin ruta y sin motivo, y ejecuta:

```bash
go test ./tests/contract/ -run ChannelMatrix
```

**Esperado**: falla nombrando agente, tipo de canal y ámbito de la celda incompleta. Retira la
celda antes de continuar. Es la comprobación de que la fuente quedó cerrada (C1, C2).

## 3. La batería no modifica el entorno de la persona

```bash
INV_ANTES=$(find "$HOME/.config/opencode" "$HOME/.claude" "$HOME/.codex" -type f 2>/dev/null | sort)
go test ./...
INV_DESPUES=$(find "$HOME/.config/opencode" "$HOME/.claude" "$HOME/.codex" -type f 2>/dev/null | sort)
diff <(echo "$INV_ANTES") <(echo "$INV_DESPUES") && echo "OK: entorno intacto"
```

**Esperado**: `OK: entorno intacto`. Es la validación de C5, el contrato que faltaba cuando la
batería llegó a eliminar un complemento real.

## 4. Simetría entre instalar y desinstalar, contra el binario

```bash
go build -o /tmp/mem_022 ./infrastructure
TMP=$(mktemp -d); HOMEV=$(mktemp -d); mkdir -p "$TMP/proy"
export GOMEMORY_DATA_HOME="$TMP/store"

HOME=$HOMEV /tmp/mem_022 install "$TMP/proy" >/dev/null
find "$TMP/proy" -type f -not -path "*/.memory/*" -not -name mem | sort > /tmp/tras_install.txt

cd "$TMP/proy" && HOME=$HOMEV ./mem uninstall . --yes >/dev/null
find "$TMP/proy" -type f -not -path "*/.memory/*" -not -name mem | sort > /tmp/tras_uninstall.txt

echo "--- artefactos que sobreviven a la desinstalación ---"
comm -12 /tmp/tras_install.txt /tmp/tras_uninstall.txt
```

**Esperado**: la lista sale vacía. Cualquier línea es un artefacto que la instalación escribe y
la desinstalación no retira (C3, SC-003).

## 5. Selección de agentes respetada

```bash
H=$(mktemp -d)
HOME=$H /tmp/mem_022 setup-mcp --scope global --agents codex >/dev/null 2>&1
find "$H" -type f | sed "s|$H|<HOME>|" | sort
```

**Esperado**: solo artefactos del agente solicitado. Antes de esta feature aparecían tres
archivos de otro agente (C8, SC-005).

## 6. El diagnóstico se deriva de la matriz

```bash
cd "$TMP/proy" && HOME=$HOMEV ./mem doctor --json | head -5
```

**Esperado**: el informe enumera exactamente las celdas de la matriz, sin lista propia (C2,
SC-007).

## Limpieza

```bash
rm -rf "$TMP" "$HOMEV" "$H" /tmp/mem_022 /tmp/tras_install.txt /tmp/tras_uninstall.txt
```
