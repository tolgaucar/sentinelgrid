"""
BÜTÜN - Dependency Graph (Bağımlılık Grafiği)

5 ana küme (ULAK, GÖRGÜ, BUYRUK, KOVAN, KIN) ve 60 modül arasındaki
bağımlılıkları yönetir. Döngüsel bağımlılık tespiti ve etki analizi yapar.
"""

from __future__ import annotations

import structlog
from collections import defaultdict, deque

logger = structlog.get_logger(__name__)


class DependencyGraph:
    """
    Bağımlılık grafiği.

    Kümeler ve modüller arası bağımlılıkları DAG (Directed Acyclic Graph)
    olarak modelleyerek, topolojik sıralama ve etki analizi yapar.
    """

    def __init__(self):
        self._edges: dict[str, set[str]] = defaultdict(set)  # node -> depends_on
        self._reverse: dict[str, set[str]] = defaultdict(set)  # node -> depended_by
        self._metadata: dict[str, dict] = {}

    def add_node(self, name: str, metadata: dict | None = None) -> None:
        """Düğüm (modül/küme) ekle."""
        if name not in self._edges:
            self._edges[name] = set()
        if metadata:
            self._metadata[name] = metadata

    def add_dependency(self, node: str, depends_on: str) -> bool:
        """
        Bağımlılık ekle: `node` -> `depends_on`

        Döngüsel bağımlılık oluşacaksa eklemeyi reddeder.
        """
        self.add_node(node)
        self.add_node(depends_on)

        # Döngüsel bağımlılık kontrolü
        if self._would_create_cycle(node, depends_on):
            logger.error("döngüsel_bağımlılık_engellendi", node=node, depends_on=depends_on)
            return False

        self._edges[node].add(depends_on)
        self._reverse[depends_on].add(node)
        logger.debug("bağımlılık_eklendi", node=node, depends_on=depends_on)
        return True

    def remove_dependency(self, node: str, depends_on: str) -> bool:
        """Bağımlılık kaldır."""
        if depends_on in self._edges.get(node, set()):
            self._edges[node].discard(depends_on)
            self._reverse[depends_on].discard(node)
            return True
        return False

    def get_dependencies(self, node: str) -> set[str]:
        """Bir düğümün doğrudan bağımlılıkları."""
        return set(self._edges.get(node, set()))

    def get_dependents(self, node: str) -> set[str]:
        """Bir düğüme bağımlı olan düğümler."""
        return set(self._reverse.get(node, set()))

    def get_all_dependencies(self, node: str) -> set[str]:
        """Bir düğümün tüm geçişli (transitive) bağımlılıkları."""
        visited = set()
        queue = deque(self._edges.get(node, set()))

        while queue:
            current = queue.popleft()
            if current in visited:
                continue
            visited.add(current)
            queue.extend(self._edges.get(current, set()) - visited)

        return visited

    def get_impact_zone(self, node: str) -> set[str]:
        """
        Etki analizi: Bir düğüm çökerse hangi düğümler etkilenir?

        Ters grafik üzerinde BFS ile tüm bağımlı düğümleri bulur.
        """
        visited = set()
        queue = deque(self._reverse.get(node, set()))

        while queue:
            current = queue.popleft()
            if current in visited:
                continue
            visited.add(current)
            queue.extend(self._reverse.get(current, set()) - visited)

        return visited

    def topological_sort(self) -> list[str]:
        """
        Kahn algoritması ile topolojik sıralama.

        Başlatma sırası: bağımlılığı olmayan düğümler önce.
        """
        # Tüm düğümleri topla (edges + deps)
        all_nodes = set(self._edges.keys())
        for node, deps in self._edges.items():
            all_nodes.update(deps)

        # in_degree: düğümün bağımlılık sayısı
        in_degree: dict[str, int] = {}
        for node in all_nodes:
            in_degree[node] = len(self._edges.get(node, set()))

        queue = deque([n for n in all_nodes if in_degree[n] == 0])
        result = []

        while queue:
            current = queue.popleft()
            result.append(current)

            for dependent in self._reverse.get(current, set()):
                in_degree[dependent] -= 1
                if in_degree[dependent] == 0:
                    queue.append(dependent)

        if len(result) != len(all_nodes):
            logger.error("topolojik_sıralama_başarısız_döngü_var")
            raise ValueError("Graf döngüsel, topolojik sıralama yapılamaz")

        return result

    def has_cycle(self) -> bool:
        """Grafta döngü var mı kontrol et."""
        try:
            self.topological_sort()
            return False
        except ValueError:
            return True

    def get_layers(self) -> list[list[str]]:
        """
        Bağımlılık katmanlarını hesapla.

        Her katmandaki düğümler paralel olarak başlatılabilir.
        """
        remaining = dict(self._edges)
        all_nodes = set(remaining.keys())
        for deps in remaining.values():
            all_nodes.update(deps)

        layers: list[list[str]] = []
        resolved: set[str] = set()

        while all_nodes - resolved:
            layer = []
            for node in all_nodes - resolved:
                deps = self._edges.get(node, set())
                if deps.issubset(resolved):
                    layer.append(node)

            if not layer:
                raise ValueError("Döngüsel bağımlılık - katman hesaplanamaz")

            layers.append(sorted(layer))
            resolved.update(layer)

        return layers

    def _would_create_cycle(self, node: str, depends_on: str) -> bool:
        """Bağımlılık eklenirse döngü oluşur mu?"""
        if node == depends_on:
            return True

        # depends_on'dan node'a ulaşılabilir mi kontrol et
        visited = set()
        queue = deque([node])

        while queue:
            current = queue.popleft()
            if current == depends_on:
                return False  # Zaten bu yönde, ters yönü kontrol etmeliyiz
            visited.add(current)

        # depends_on'dan node'a ulaşılabilirlik kontrolü
        visited.clear()
        queue = deque([depends_on])
        while queue:
            current = queue.popleft()
            if current in visited:
                continue
            if current == node:
                return True
            visited.add(current)
            queue.extend(self._edges.get(current, set()) - visited)

        return False

    def visualize_text(self) -> str:
        """Grafın metin gösterimi."""
        lines = ["=== ÖTÜKEN-MATRİS Bağımlılık Grafiği ===", ""]
        for node in sorted(self._edges.keys()):
            deps = self._edges[node]
            dep_str = ", ".join(sorted(deps)) if deps else "(bağımsız)"
            lines.append(f"  {node} → {dep_str}")
        return "\n".join(lines)
