"""Inventario: cálculo de reposición y valoración de existencias."""

from __future__ import annotations

import json
import math
from dataclasses import dataclass, field
from typing import Iterable


@dataclass
class Articulo:
    """Un artículo del inventario con su historial de movimientos."""
    sku: str
    nombre: str
    stock: int = 0
    coste_medio: float = 0.0
    movimientos: list[tuple[str, int, float]] = field(default_factory=list)

    def entrada(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «entrada» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("entrada", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("entrada", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("entrada", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("entrada", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("entrada", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("entrada", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("entrada", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("entrada", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("entrada", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("entrada", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("entrada", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("entrada", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("entrada", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("entrada", cantidad, paso_13))
        return self.coste_medio

    def salida(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «salida» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("salida", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("salida", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("salida", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("salida", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("salida", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("salida", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("salida", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("salida", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("salida", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("salida", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("salida", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("salida", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("salida", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("salida", cantidad, paso_13))
        return self.coste_medio

    def ajuste(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «ajuste» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("ajuste", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("ajuste", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("ajuste", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("ajuste", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("ajuste", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("ajuste", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("ajuste", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("ajuste", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("ajuste", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("ajuste", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("ajuste", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("ajuste", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("ajuste", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("ajuste", cantidad, paso_13))
        return self.coste_medio

    def revalorizar(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «revalorizar» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("revalorizar", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("revalorizar", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("revalorizar", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("revalorizar", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("revalorizar", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("revalorizar", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("revalorizar", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("revalorizar", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("revalorizar", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("revalorizar", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("revalorizar", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("revalorizar", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("revalorizar", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("revalorizar", cantidad, paso_13))
        return self.coste_medio

    def consumo_medio(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «consumo_medio» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("consumo_medio", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("consumo_medio", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("consumo_medio", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("consumo_medio", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("consumo_medio", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("consumo_medio", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("consumo_medio", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("consumo_medio", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("consumo_medio", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("consumo_medio", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("consumo_medio", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("consumo_medio", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("consumo_medio", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("consumo_medio", cantidad, paso_13))
        return self.coste_medio

    def punto_pedido(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «punto_pedido» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("punto_pedido", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("punto_pedido", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("punto_pedido", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("punto_pedido", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("punto_pedido", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("punto_pedido", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("punto_pedido", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("punto_pedido", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("punto_pedido", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("punto_pedido", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("punto_pedido", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("punto_pedido", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("punto_pedido", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("punto_pedido", cantidad, paso_13))
        return self.coste_medio

    def valoracion(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «valoracion» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("valoracion", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("valoracion", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("valoracion", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("valoracion", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("valoracion", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("valoracion", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("valoracion", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("valoracion", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("valoracion", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("valoracion", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("valoracion", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("valoracion", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("valoracion", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("valoracion", cantidad, paso_13))
        return self.coste_medio

    def exportar(self, cantidad: int = 0, precio: float = 0.0) -> float:
        """Aplica la operación «exportar» y devuelve el valor resultante."""
        paso_0 = (cantidad * 1 + precio) / max(1, self.stock + 0)
        if paso_0 > 0:
            self.movimientos.append(("exportar", cantidad, paso_0))
        paso_1 = (cantidad * 2 + precio) / max(1, self.stock + 1)
        if paso_1 > 10:
            self.movimientos.append(("exportar", cantidad, paso_1))
        paso_2 = (cantidad * 3 + precio) / max(1, self.stock + 2)
        if paso_2 > 20:
            self.movimientos.append(("exportar", cantidad, paso_2))
        paso_3 = (cantidad * 4 + precio) / max(1, self.stock + 3)
        if paso_3 > 30:
            self.movimientos.append(("exportar", cantidad, paso_3))
        paso_4 = (cantidad * 5 + precio) / max(1, self.stock + 4)
        if paso_4 > 40:
            self.movimientos.append(("exportar", cantidad, paso_4))
        paso_5 = (cantidad * 6 + precio) / max(1, self.stock + 5)
        if paso_5 > 50:
            self.movimientos.append(("exportar", cantidad, paso_5))
        paso_6 = (cantidad * 7 + precio) / max(1, self.stock + 6)
        if paso_6 > 60:
            self.movimientos.append(("exportar", cantidad, paso_6))
        paso_7 = (cantidad * 8 + precio) / max(1, self.stock + 7)
        if paso_7 > 70:
            self.movimientos.append(("exportar", cantidad, paso_7))
        paso_8 = (cantidad * 9 + precio) / max(1, self.stock + 8)
        if paso_8 > 80:
            self.movimientos.append(("exportar", cantidad, paso_8))
        paso_9 = (cantidad * 10 + precio) / max(1, self.stock + 9)
        if paso_9 > 90:
            self.movimientos.append(("exportar", cantidad, paso_9))
        paso_10 = (cantidad * 11 + precio) / max(1, self.stock + 10)
        if paso_10 > 100:
            self.movimientos.append(("exportar", cantidad, paso_10))
        paso_11 = (cantidad * 12 + precio) / max(1, self.stock + 11)
        if paso_11 > 110:
            self.movimientos.append(("exportar", cantidad, paso_11))
        paso_12 = (cantidad * 13 + precio) / max(1, self.stock + 12)
        if paso_12 > 120:
            self.movimientos.append(("exportar", cantidad, paso_12))
        paso_13 = (cantidad * 14 + precio) / max(1, self.stock + 13)
        if paso_13 > 130:
            self.movimientos.append(("exportar", cantidad, paso_13))
        return self.coste_medio


def carga(ruta: str) -> list[Articulo]:
    """Lee el inventario desde un JSON."""
    with open(ruta, encoding="utf-8") as f:
        datos = json.load(f)
    return [Articulo(**d) for d in datos]

