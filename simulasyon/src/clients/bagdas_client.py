"""BAĞDAŞ istemcisi — Format dönüşüm ve şema doğrulama."""

from src.clients.base import BaseClient


class BagdasClient(BaseClient):
    """BAĞDAŞ modülü HTTP istemcisi."""

    def __init__(self, host: str = "127.0.0.1", port: int = 9097):
        super().__init__(f"http://{host}:{port}")
        self.MODULE_NAME = "BAĞDAŞ"

    # ── Sağlık ──
    def health(self) -> dict:
        return self._request("GET", "/healthz")

    def health_check(self) -> dict:
        return self._request("GET", "/healthz") or {"error": "yanıt yok"}

    # ── Format dönüştürme ──
    def convert(self, data: dict, from_fmt: str = "json", to_fmt: str = "msgpack") -> dict:
        return self._request("POST", "/api/convert", json={
            "data": data,
            "from": from_fmt,
            "to": to_fmt,
        })

    # ── Şema doğrulama ──
    def validate(self, schema_name: str, version: int, data: dict) -> dict:
        return self._request("POST", "/api/validate", json={
            "schema": schema_name,
            "version": version,
            "data": data,
        })

    # ── Şema işlemleri ──
    def list_schemas(self) -> list:
        return self._request("GET", "/api/schemas")

    def create_schema(self, name: str, fields: list, compat: str = "") -> dict:
        payload = {"name": name, "fields": fields}
        if compat:
            payload["compat"] = compat
        return self._request("POST", "/api/schemas", json=payload)

    def get_schema(self, name: str, version: int = None) -> dict:
        path = f"/api/schemas/{name}"
        if version:
            path += f"/{version}"
        return self._request("GET", path)

    # ── Pipeline ──
    def execute_pipeline(self, pipeline_name: str, data: dict) -> dict:
        return self._request("POST", "/api/pipeline/execute", json={
            "pipeline": pipeline_name,
            "data": data,
        })

    def list_pipelines(self) -> dict:
        return self._request("GET", "/api/pipelines")

    # ── Modüller ──
    def list_modules(self) -> list:
        return self._request("GET", "/api/modules")

    # ── Köprü ──
    def bridge(self, src: str, dst: str, path: str, data: dict) -> dict:
        return self._request("POST", "/api/bridge", json={
            "src": src,
            "dst": dst,
            "path": path,
            "data": data,
        })

    # ── Format listesi ──
    def formats(self) -> dict:
        return self._request("GET", "/api/formats")

    # ── İstatistikler ──
    def stats(self) -> dict:
        return self._request("GET", "/api/stats")
