from __future__ import annotations
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional
import numpy as np


@dataclass
class BBox:
    x: int
    y: int
    w: int
    h: int

    def as_tuple(self) -> tuple[int, int, int, int]:
        return self.x, self.y, self.w, self.h


@dataclass
class Detection:
    type: str                          # "motion" | "face" | "object"
    label: str                         # e.g. "stranger", "Alice", "person"
    confidence: float
    bbox: Optional[BBox] = None
    timestamp: datetime = field(default_factory=datetime.now)
    extra: dict = field(default_factory=dict)


class Detector(ABC):
    @abstractmethod
    def process(self, frame: np.ndarray) -> list[Detection]:
        ...

    def close(self) -> None:
        pass
