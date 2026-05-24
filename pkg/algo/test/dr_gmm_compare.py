from __future__ import annotations

import argparse
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Any

BASE_DIR = Path(__file__).resolve().parent
MPLCONFIGDIR = Path(tempfile.gettempdir()) / "gonotelm-mplconfig"
MPLCONFIGDIR.mkdir(parents=True, exist_ok=True)
os.environ.setdefault("MPLCONFIGDIR", str(MPLCONFIGDIR))

import matplotlib.pyplot as plt
import numpy as np
from sklearn.decomposition import PCA
from sklearn.manifold import TSNE, trustworthiness
from sklearn.metrics import adjusted_rand_score
from sklearn.mixture import GaussianMixture

try:
    import umap
except ModuleNotFoundError as exc:  # pragma: no cover - runtime dependency guard
    raise ModuleNotFoundError(
        "Missing python dependency `umap-learn`. Install with:\n"
        "uv pip install --python ../.venv/bin/python umap-learn"
    ) from exc


def to_plot_space(points: np.ndarray) -> np.ndarray:
    if points.ndim != 2:
        raise ValueError("points must be a 2D array")
    if points.shape[1] >= 2:
        return points[:, :2]
    if points.shape[1] == 1:
        return np.hstack([points, np.zeros((points.shape[0], 1), dtype=points.dtype)])
    raise ValueError("points must have at least one feature")


def condensed_pairwise_distances(points: np.ndarray) -> np.ndarray:
    n_samples = points.shape[0]
    distances: list[float] = []
    for i in range(n_samples):
        for j in range(i + 1, n_samples):
            distances.append(float(np.linalg.norm(points[i] - points[j])))
    return np.asarray(distances, dtype=np.float64)


def normalize_by_mean(values: np.ndarray) -> np.ndarray:
    if values.size == 0:
        return values.copy()
    mean_value = float(np.mean(values))
    if mean_value == 0:
        return values.copy()
    return values / mean_value


def l2_normalize_rows(vectors: np.ndarray, epsilon: float = 1e-12) -> np.ndarray:
    if vectors.ndim != 2:
        raise ValueError("vectors must be a 2D array")
    normalized = vectors.astype(np.float64, copy=True)
    row_norms = np.linalg.norm(normalized, ord=2, axis=1)
    safe = row_norms > float(epsilon)
    if np.any(safe):
        normalized[safe] = normalized[safe] / row_norms[safe, np.newaxis]
    return normalized


def pearson_correlation(a: np.ndarray, b: np.ndarray) -> float:
    if a.size == 0 or b.size == 0 or a.size != b.size:
        return 0.0
    if float(np.std(a)) == 0.0 or float(np.std(b)) == 0.0:
        return 0.0
    return float(np.corrcoef(a, b)[0, 1])


def canonicalize_method_result(result: dict[str, Any]) -> dict[str, Any]:
    means = np.asarray(result["means"], dtype=np.float64)
    if means.ndim != 2 or means.shape[0] == 0:
        return result

    sort_keys = tuple(means[:, idx] for idx in range(means.shape[1] - 1, -1, -1))
    order = np.lexsort(sort_keys)

    remap = np.empty(order.shape[0], dtype=np.int64)
    remap[order] = np.arange(order.shape[0], dtype=np.int64)

    labels = np.asarray(result["labels"], dtype=np.int64)
    result["labels"] = remap[labels].tolist()
    result["means"] = means[order].tolist()
    return result


def fit_gmm_auto_bic(embedding: np.ndarray, seed: int) -> tuple[GaussianMixture, np.ndarray]:
    n_samples = embedding.shape[0]
    min_k = 1
    max_k = min(10, n_samples)
    best_model: GaussianMixture | None = None
    best_labels: np.ndarray | None = None
    best_score = float("inf")
    for k in range(min_k, max_k + 1):
        model = GaussianMixture(
            n_components=k,
            covariance_type="full",
            tol=1e-3,
            reg_covar=1e-6,
            max_iter=100,
            n_init=1,
            init_params="kmeans",
            random_state=seed,
        )
        labels = model.fit_predict(embedding)
        score = float(model.bic(embedding))
        if score < best_score:
            best_score = score
            best_model = model
            best_labels = labels
    if best_model is None or best_labels is None:
        raise RuntimeError("auto GMM selection failed")
    return best_model, best_labels


def build_method_result(
    method_name: str,
    vectors: np.ndarray,
    embedding: np.ndarray,
    normalized_high_condensed: np.ndarray,
    trust_neighbors: int,
    cluster_count: int,
    seed: int,
    auto_cluster: bool = False,
) -> dict[str, Any]:
    if auto_cluster:
        gmm, labels = fit_gmm_auto_bic(embedding, seed)
    else:
        gmm = GaussianMixture(
            n_components=cluster_count,
            covariance_type="full",
            tol=1e-3,
            reg_covar=1e-6,
            max_iter=100,
            n_init=1,
            init_params="kmeans",
            random_state=seed,
        )
        labels = gmm.fit_predict(embedding)
    low_condensed = condensed_pairwise_distances(embedding)
    normalized_low = normalize_by_mean(low_condensed)

    n_samples = vectors.shape[0]
    neighbors = min(max(1, trust_neighbors), n_samples - 1)
    trust = float(trustworthiness(vectors, embedding, n_neighbors=neighbors, metric="euclidean"))
    dist_corr = pearson_correlation(normalized_high_condensed, normalized_low)

    result = {
        "name": method_name,
        "embedding": embedding.tolist(),
        "labels": labels.tolist(),
        "means": np.asarray(gmm.means_, dtype=np.float64).tolist(),
        "trustworthiness": trust,
        "distance_correlation": dist_corr,
        "log_likelihood": float(gmm.score(embedding) * n_samples),
        "bic": float(gmm.bic(embedding)),
    }
    return canonicalize_method_result(result)


def compute_snapshot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    vectors = np.asarray(payload["vectors"], dtype=np.float64)
    if vectors.ndim != 2 or vectors.shape[0] < 2:
        raise ValueError("payload.vectors must be a 2D array with at least 2 samples")

    seed = int(payload.get("seed", 42))
    trust_neighbors = int(payload.get("trust_neighbors", 10))
    cluster_count = int(payload.get("cluster_count", 4))
    cluster_count = min(max(2, cluster_count), vectors.shape[0])
    auto_cluster = bool(payload.get("auto_cluster", False))
    normalize_l2 = bool(payload.get("normalize_l2", False))
    tsne_perplexity = float(payload["tsne_perplexity"])
    tsne_perplexity = max(1.0, min(tsne_perplexity, float(vectors.shape[0] - 1)))

    if normalize_l2:
        vectors = l2_normalize_rows(vectors)

    high_condensed = condensed_pairwise_distances(vectors)
    normalized_high = normalize_by_mean(high_condensed)

    pca = PCA(n_components=2, svd_solver="full", whiten=False)
    pca_embedding = np.asarray(pca.fit_transform(vectors), dtype=np.float64)

    tsne = TSNE(
        n_components=2,
        perplexity=tsne_perplexity,
        early_exaggeration=12.0,
        learning_rate="auto",
        max_iter=350,
        n_iter_without_progress=200,
        min_grad_norm=1e-7,
        init="pca",
        method="exact",
        random_state=seed,
    )
    tsne_embedding = np.asarray(tsne.fit_transform(vectors), dtype=np.float64)

    umap_model = umap.UMAP(
        n_components=2,
        n_neighbors=15,
        min_dist=0.1,
        spread=1.0,
        metric="euclidean",
        init="spectral",
        random_state=seed,
        n_jobs=1,
    )
    umap_embedding = np.asarray(umap_model.fit_transform(vectors), dtype=np.float64)

    methods = [
        build_method_result(
            "PCA",
            vectors,
            pca_embedding,
            normalized_high,
            trust_neighbors,
            cluster_count,
            seed,
            auto_cluster,
        ),
        build_method_result(
            "t-SNE",
            vectors,
            tsne_embedding,
            normalized_high,
            trust_neighbors,
            cluster_count,
            seed,
            auto_cluster,
        ),
        build_method_result(
            "UMAP",
            vectors,
            umap_embedding,
            normalized_high,
            trust_neighbors,
            cluster_count,
            seed,
            auto_cluster,
        ),
    ]
    return {"methods": methods}


def plot_panel(
    ax: plt.Axes,
    embedding: np.ndarray,
    labels: np.ndarray,
    means: np.ndarray,
    title: str,
) -> None:
    ax.scatter(
        embedding[:, 0],
        embedding[:, 1],
        c=labels,
        cmap="tab10",
        s=14,
        alpha=0.78,
        linewidths=0.0,
    )
    ax.scatter(
        means[:, 0],
        means[:, 1],
        c=np.arange(means.shape[0], dtype=np.int64),
        cmap="tab10",
        marker="X",
        s=120,
        edgecolors="black",
        linewidths=1.0,
    )
    ax.set_title(title, fontsize=9)
    ax.set_xlabel("Dim 1")
    ax.set_ylabel("Dim 2")
    ax.grid(alpha=0.2, linewidth=0.5)


def render_go_vs_python_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    dataset_name = str(payload.get("dataset_name", "dataset"))
    go_methods = payload.get("go_methods")
    python_methods = payload.get("python_methods")
    if not isinstance(go_methods, list) or not isinstance(python_methods, list):
        raise ValueError("payload.go_methods and payload.python_methods must be lists")

    go_by_name = {str(item["name"]): item for item in go_methods}
    py_by_name = {str(item["name"]): item for item in python_methods}
    method_order = ["PCA", "t-SNE", "UMAP"]

    fig, axes = plt.subplots(len(method_order), 2, figsize=(13, 4.1 * len(method_order)), dpi=150, squeeze=False)

    for row_idx, method_name in enumerate(method_order):
        if method_name not in go_by_name or method_name not in py_by_name:
            raise ValueError(f"missing method in payload: {method_name}")

        py_item = canonicalize_method_result(dict(py_by_name[method_name]))
        go_item = canonicalize_method_result(dict(go_by_name[method_name]))

        py_embedding = to_plot_space(np.asarray(py_item["embedding"], dtype=np.float64))
        go_embedding = to_plot_space(np.asarray(go_item["embedding"], dtype=np.float64))
        py_labels = np.asarray(py_item["labels"], dtype=np.int64)
        go_labels = np.asarray(go_item["labels"], dtype=np.int64)
        py_means = to_plot_space(np.asarray(py_item["means"], dtype=np.float64))
        go_means = to_plot_space(np.asarray(go_item["means"], dtype=np.float64))

        ari = float(adjusted_rand_score(py_labels, go_labels))

        py_k = int(np.asarray(py_item["means"], dtype=np.float64).shape[0])
        go_k = int(np.asarray(go_item["means"], dtype=np.float64).shape[0])

        py_title = (
            f"Python {method_name} + GMM\n"
            f"K={py_k} "
            f"Trust={float(py_item['trustworthiness']):.3f} "
            f"DistCorr={float(py_item['distance_correlation']):.3f} "
            f"BIC={float(py_item['bic']):.1f}"
        )
        go_title = (
            f"Go {method_name} + GMM\n"
            f"K={go_k} "
            f"Trust={float(go_item['trustworthiness']):.3f} "
            f"DistCorr={float(go_item['distance_correlation']):.3f} "
            f"BIC={float(go_item['bic']):.1f} "
            f"ARI(py,go)={ari:.3f}"
        )

        plot_panel(axes[row_idx, 0], py_embedding, py_labels, py_means, py_title)
        plot_panel(axes[row_idx, 1], go_embedding, go_labels, go_means, go_title)

    fig.suptitle(f"{dataset_name}: DR + GaussianMixture (Python vs Go)", fontsize=12)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / f"dr_gmm_compare_go_vs_python_{dataset_name}.png")
    if not output_path.is_absolute():
        output_path = BASE_DIR / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)
    return {"output_file": str(output_path)}


def stitch_comparison_images_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    datasets = payload.get("datasets")
    if not isinstance(datasets, list) or len(datasets) == 0:
        raise ValueError("payload.datasets must be a non-empty list")

    n_rows = len(datasets)
    fig, axes = plt.subplots(n_rows, 1, figsize=(14, 4.8 * n_rows), dpi=150, squeeze=False)

    for row_idx, dataset in enumerate(datasets):
        name = str(dataset.get("name", f"dataset_{row_idx + 1}"))
        image_file = dataset.get("image_file")
        if not image_file:
            raise ValueError(f"datasets[{row_idx}] missing image_file")
        image_path = Path(image_file)
        if not image_path.is_absolute():
            image_path = BASE_DIR / image_path
        if not image_path.exists():
            raise FileNotFoundError(f"comparison image not found: {image_path}")

        image = plt.imread(image_path)
        ax = axes[row_idx, 0]
        ax.imshow(image)
        ax.axis("off")
        ax.set_title(name, fontsize=11, pad=8)

    title = str(payload.get("title", "DR + GMM comparison overview"))
    fig.suptitle(title, fontsize=13)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / "dr_gmm_compare_overview.png")
    if not output_path.is_absolute():
        output_path = BASE_DIR / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)
    return {"output_file": str(output_path)}


class TestDrGMMCompare(unittest.TestCase):
    def test_compute_snapshot_from_payload(self) -> None:
        rng = np.random.default_rng(42)
        vectors = rng.normal(size=(40, 8)).astype(np.float64)
        payload = {
            "vectors": vectors.tolist(),
            "tsne_perplexity": 10.0,
            "trust_neighbors": 8,
            "cluster_count": 4,
            "seed": 42,
        }
        result = compute_snapshot_from_payload(payload)
        methods = result["methods"]
        self.assertEqual([item["name"] for item in methods], ["PCA", "t-SNE", "UMAP"])
        for method in methods:
            embedding = np.asarray(method["embedding"], dtype=np.float64)
            labels = np.asarray(method["labels"], dtype=np.int64)
            means = np.asarray(method["means"], dtype=np.float64)
            self.assertEqual(embedding.shape, (40, 2))
            self.assertEqual(labels.shape, (40,))
            self.assertEqual(means.shape[0], 4)

    def test_render_go_vs_python_plot_from_payload(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dr-gmm-go-vs-py-") as temp_dir:
            output_path = Path(temp_dir) / "compare.png"
            base_method = {
                "embedding": [[0.0, 0.0], [1.0, 1.0], [1.1, 0.9], [3.0, 3.0]],
                "labels": [0, 0, 0, 1],
                "means": [[0.7, 0.63], [3.0, 3.0]],
                "trustworthiness": 0.9,
                "distance_correlation": 0.8,
                "log_likelihood": -12.0,
                "bic": 34.0,
            }
            payload = {
                "dataset_name": "smoke",
                "python_methods": [
                    {"name": "PCA", **base_method},
                    {"name": "t-SNE", **base_method},
                    {"name": "UMAP", **base_method},
                ],
                "go_methods": [
                    {"name": "PCA", **base_method},
                    {"name": "t-SNE", **base_method},
                    {"name": "UMAP", **base_method},
                ],
                "output_file": str(output_path),
            }
            result = render_go_vs_python_plot_from_payload(payload)
            self.assertEqual(result["output_file"], str(output_path))
            self.assertTrue(output_path.exists())

    def test_stitch_comparison_images_from_payload(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dr-gmm-stitch-") as temp_dir:
            temp_path = Path(temp_dir)
            image_a = temp_path / "a.png"
            image_b = temp_path / "b.png"

            plt.figure(figsize=(2, 1), dpi=80)
            plt.plot([0, 1], [0, 1], color="blue")
            plt.tight_layout()
            plt.savefig(image_a, bbox_inches="tight")
            plt.close()

            plt.figure(figsize=(2, 1), dpi=80)
            plt.plot([0, 1], [1, 0], color="orange")
            plt.tight_layout()
            plt.savefig(image_b, bbox_inches="tight")
            plt.close()

            output_path = temp_path / "overview.png"
            payload = {
                "title": "overview",
                "datasets": [
                    {"name": "a", "image_file": str(image_a)},
                    {"name": "b", "image_file": str(image_b)},
                ],
                "output_file": str(output_path),
            }

            result = stitch_comparison_images_from_payload(payload)
            self.assertEqual(result["output_file"], str(output_path))
            self.assertTrue(output_path.exists())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--snapshot-stdin",
        action="store_true",
        help="Read vectors payload from stdin JSON and output Python DR+GMM snapshot.",
    )
    parser.add_argument(
        "--plot-stdin",
        action="store_true",
        help="Read Go/Python payload from stdin JSON and render DR+GMM comparison plot.",
    )
    parser.add_argument(
        "--stitch-stdin",
        action="store_true",
        help="Read stitch payload from stdin JSON and render overview image.",
    )
    args, remaining = parser.parse_known_args()

    if args.snapshot_stdin:
        payload = json.load(sys.stdin)
        result = compute_snapshot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.plot_stdin:
        payload = json.load(sys.stdin)
        result = render_go_vs_python_plot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.stitch_stdin:
        payload = json.load(sys.stdin)
        result = stitch_comparison_images_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    unittest.main(argv=[sys.argv[0], *remaining])


if __name__ == "__main__":
    main()
