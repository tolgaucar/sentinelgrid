from setuptools import setup, find_packages

setup(
    name="butun",
    version="0.1.0",
    description="BÜTÜN - ÖTÜKEN-MATRİS Ana Çatı Sistemi",
    packages=find_packages(),
    python_requires=">=3.11",
    install_requires=[
        "fastapi>=0.110.0",
        "uvicorn>=0.27.0",
        "pydantic>=2.5.0",
        "pyyaml>=6.0.1",
        "httpx>=0.27.0",
        "psutil>=5.9.0",
        "structlog>=24.1.0",
    ],
    entry_points={
        "console_scripts": [
            "butun=src.main:main",
        ],
    },
)
