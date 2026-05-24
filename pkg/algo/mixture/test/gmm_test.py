from __future__ import annotations

import argparse
import copy
import json
import os
import tempfile
import sys
import unittest
from pathlib import Path
from typing import Any

BASE_DIR = Path(__file__).resolve().parent
MPLCONFIGDIR = Path(tempfile.gettempdir()) / "gonotelm-mplconfig"
MPLCONFIGDIR.mkdir(parents=True, exist_ok=True)
os.environ.setdefault("MPLCONFIGDIR", str(MPLCONFIGDIR))

import matplotlib.pyplot as plt
import numpy as np
from sklearn.metrics import adjusted_rand_score
from sklearn.mixture import GaussianMixture

TESTDATA_DIR = BASE_DIR / "testdata"
CASES_FILE = TESTDATA_DIR / "gmm_cases.json"


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def load_cases() -> list[dict[str, Any]]:
    cases = json.loads(CASES_FILE.read_text(encoding="utf-8"))
    if not isinstance(cases, list) or not cases:
        raise ValueError(f"invalid gmm cases file: {CASES_FILE}")
    return cases


def get_case_by_name(case_name: str) -> dict[str, Any]:
    for case in load_cases():
        if case["name"] == case_name:
            return case
    raise ValueError(f"case not found: {case_name}")


def load_vectors_from_jsonl(path: Path) -> np.ndarray:
    rows: list[list[float]] = []
    with path.open("r", encoding="utf-8") as file:
        for raw_line in file:
            line = raw_line.strip()
            if not line:
                continue
            rows.append(json.loads(line))
    if not rows:
        raise ValueError(f"jsonl dataset is empty: {path}")
    return np.asarray(rows, dtype=np.float64)


def load_dataset_for_case(case: dict[str, Any]) -> tuple[np.ndarray, np.ndarray | None]:
    source_path = TESTDATA_DIR / case["source_file"]
    kind = case["kind"]
    if kind == "payload_json":
        payload = load_json(source_path)
        vectors = np.asarray(payload["vectors"], dtype=np.float64)
        true_labels = payload.get("true_labels")
        labels = np.asarray(true_labels, dtype=np.int64) if true_labels is not None else None
    elif kind == "jsonl":
        vectors = load_vectors_from_jsonl(source_path)
        labels = None
    else:
        raise ValueError(f"unsupported case kind: {kind}")

    feature_limit = int(case.get("feature_limit", 0))
    if feature_limit > 0 and vectors.shape[1] > feature_limit:
        vectors = vectors[:, :feature_limit].copy()
    return vectors, labels


def canonicalize_snapshot(snapshot: dict[str, Any]) -> dict[str, Any]:
    means = np.asarray(snapshot["means"], dtype=np.float64)
    if means.ndim != 2 or means.shape[0] == 0:
        return snapshot

    sort_keys = tuple(means[:, idx] for idx in range(means.shape[1] - 1, -1, -1))
    order = np.lexsort(sort_keys)
    remap = np.empty(order.shape[0], dtype=np.int64)
    remap[order] = np.arange(order.shape[0], dtype=np.int64)

    labels = np.asarray(snapshot["labels"], dtype=np.int64)
    snapshot["labels"] = remap[labels].tolist()
    snapshot["weights"] = np.asarray(snapshot["weights"], dtype=np.float64)[order].tolist()
    snapshot["means"] = means[order].tolist()
    return snapshot


def fit_sklearn_snapshot(case: dict[str, Any]) -> dict[str, Any]:
    vectors, true_labels = load_dataset_for_case(case)
    model = build_sklearn_model(case, int(case["cluster_count"]))
    labels = model.fit_predict(vectors)

    snapshot: dict[str, Any] = {
        "name": case["name"],
        "dataset_file": case["source_file"],
        "covariance_type": case.get("covariance_type", "full"),
        "cluster_count": int(case["cluster_count"]),
        "labels": labels.tolist(),
        "weights": model.weights_.tolist(),
        "means": model.means_.tolist(),
        "log_likelihood": float(model.score(vectors) * vectors.shape[0]),
        "bic": float(model.bic(vectors)),
        "iterations": int(model.n_iter_),
        "converged": bool(model.converged_),
    }
    if true_labels is not None:
        snapshot["ari_vs_true_labels"] = float(adjusted_rand_score(true_labels, labels))
    return canonicalize_snapshot(snapshot)


def build_sklearn_model(case: dict[str, Any], n_components: int) -> GaussianMixture:
    return GaussianMixture(
        n_components=int(n_components),
        covariance_type=case.get("covariance_type", "full"),
        tol=float(case["tolerance"]),
        reg_covar=float(case["regularization"]),
        max_iter=int(case["max_iterations"]),
        n_init=max(int(case.get("n_init", 1)), 1),
        init_params=case.get("init_params", "kmeans"),
        random_state=int(case["seed"]),
    )


def resolve_auto_range(
    n_samples: int,
    min_components: int | None,
    max_components: int | None,
) -> tuple[int, int]:
    min_k = 1 if min_components is None else int(min_components)
    max_k = min(10, n_samples) if max_components is None else int(max_components)
    if min_k <= 0:
        raise ValueError(f"min_components must be positive, got {min_k}")
    if max_k < min_k:
        raise ValueError(f"max_components must be >= min_components, got min={min_k} max={max_k}")
    if max_k > n_samples:
        raise ValueError(f"max_components={max_k} exceeds n_samples={n_samples}")
    return min_k, max_k


def fit_sklearn_snapshot_auto(
    case: dict[str, Any],
    criterion: str = "bic",
    min_components: int | None = None,
    max_components: int | None = None,
) -> dict[str, Any]:
    criterion = criterion.lower().strip()
    if criterion not in {"bic", "aic"}:
        raise ValueError(f"unsupported criterion: {criterion}")

    vectors, true_labels = load_dataset_for_case(case)
    min_k, max_k = resolve_auto_range(vectors.shape[0], min_components, max_components)
    best: dict[str, Any] | None = None
    best_score = float("inf")

    for n_components in range(min_k, max_k + 1):
        model = build_sklearn_model(case, n_components)
        labels = model.fit_predict(vectors)
        bic = float(model.bic(vectors))
        aic = float(model.aic(vectors))
        score = bic if criterion == "bic" else aic
        if score < best_score:
            best_score = score
            best = {
                "model": model,
                "labels": labels,
                "bic": bic,
                "aic": aic,
                "k": int(n_components),
            }

    if best is None:
        raise RuntimeError("auto selection did not produce a valid model")

    model = best["model"]
    labels = best["labels"]
    snapshot: dict[str, Any] = {
        "name": case["name"],
        "dataset_file": case["source_file"],
        "covariance_type": case.get("covariance_type", "full"),
        "cluster_count": int(best["k"]),
        "labels": labels.tolist(),
        "weights": model.weights_.tolist(),
        "means": model.means_.tolist(),
        "log_likelihood": float(model.score(vectors) * vectors.shape[0]),
        "bic": float(best["bic"]),
        "iterations": int(model.n_iter_),
        "converged": bool(model.converged_),
        "selection_criterion": criterion,
        "selection_score": float(best_score),
        "selection_min_components": int(min_k),
        "selection_max_components": int(max_k),
    }
    if true_labels is not None:
        snapshot["ari_vs_true_labels"] = float(adjusted_rand_score(true_labels, labels))
    return canonicalize_snapshot(snapshot)


def plot_panel(
    ax: plt.Axes,
    vectors: np.ndarray,
    labels: np.ndarray,
    means: np.ndarray,
    title: str,
) -> None:
    ax.scatter(
        vectors[:, 0],
        vectors[:, 1],
        c=labels,
        cmap="tab10",
        s=8,
        alpha=0.65,
        linewidths=0.0,
    )
    ax.scatter(
        means[:, 0],
        means[:, 1],
        c=np.arange(means.shape[0]),
        cmap="tab10",
        marker="X",
        s=140,
        edgecolors="black",
        linewidths=1.1,
    )
    ax.set_title(title, fontsize=10)
    ax.set_xlabel("x")
    ax.set_ylabel("y")
    ax.grid(alpha=0.2, linewidth=0.5)


def _to_plot_space(vectors: np.ndarray, means: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
    if vectors.shape[1] >= 2:
        return vectors[:, :2], means[:, :2]
    if vectors.shape[1] == 1:
        vector_2d = np.hstack([vectors, np.zeros((vectors.shape[0], 1), dtype=vectors.dtype)])
        means_2d = np.hstack([means, np.zeros((means.shape[0], 1), dtype=means.dtype)])
        return vector_2d, means_2d
    raise ValueError("vectors must contain at least one feature")


def render_comparison_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    case_name = str(payload["case_name"])
    vectors = np.asarray(payload["vectors"], dtype=np.float64)

    sklearn_snapshot = canonicalize_snapshot(payload["sklearn_snapshot"])
    go_snapshot = canonicalize_snapshot(payload["go_snapshot"])

    sklearn_labels = np.asarray(sklearn_snapshot["labels"], dtype=np.int64)
    go_labels = np.asarray(go_snapshot["labels"], dtype=np.int64)
    sklearn_means = np.asarray(sklearn_snapshot["means"], dtype=np.float64)
    go_means = np.asarray(go_snapshot["means"], dtype=np.float64)

    plot_vectors, sklearn_plot_means = _to_plot_space(vectors, sklearn_means)
    _, go_plot_means = _to_plot_space(vectors, go_means)

    ari_go_vs_sklearn = float(adjusted_rand_score(sklearn_labels, go_labels))
    ari_sklearn_vs_true = sklearn_snapshot.get("ari_vs_true_labels")
    ari_go_vs_true = go_snapshot.get("ari_vs_true_labels")

    fig, axes = plt.subplots(1, 2, figsize=(14, 6), dpi=150, sharex=True, sharey=True)

    sklearn_title = (
        "sklearn GaussianMixture\n"
        f"K={int(sklearn_snapshot['cluster_count'])}  "
        f"LL={float(sklearn_snapshot['log_likelihood']):.2f}  "
        f"BIC={float(sklearn_snapshot['bic']):.2f}"
    )
    go_title = (
        "Go GaussianMixture\n"
        f"K={int(go_snapshot['cluster_count'])}  "
        f"LL={float(go_snapshot['log_likelihood']):.2f}  "
        f"BIC={float(go_snapshot['bic']):.2f}"
    )
    if ari_sklearn_vs_true is not None:
        sklearn_title = f"{sklearn_title}  ARI(true)={float(ari_sklearn_vs_true):.4f}"
    if ari_go_vs_true is not None:
        go_title = f"{go_title}  ARI(true)={float(ari_go_vs_true):.4f}"

    plot_panel(
        axes[0],
        plot_vectors,
        sklearn_labels,
        sklearn_plot_means,
        sklearn_title,
    )
    plot_panel(
        axes[1],
        plot_vectors,
        go_labels,
        go_plot_means,
        go_title,
    )
    fig.suptitle(
        f"{case_name}: Go vs sklearn GMM comparison (ARI(go,sk)={ari_go_vs_sklearn:.4f})",
        fontsize=12,
    )
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else TESTDATA_DIR / f"gmm_compare_{case_name}.png"
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {
        "output_file": str(output_path),
        "ari_go_vs_sklearn": ari_go_vs_sklearn,
    }


class TestSklearnGaussianMixtureReference(unittest.TestCase):
    def test_all_cases_are_fittable(self) -> None:
        for case in load_cases():
            with self.subTest(case=case["name"]):
                vectors, _ = load_dataset_for_case(case)
                snapshot = fit_sklearn_snapshot(copy.deepcopy(case))
                self.assertEqual(len(snapshot["labels"]), vectors.shape[0])
                self.assertEqual(snapshot["cluster_count"], int(case["cluster_count"]))
                self.assertTrue(np.isfinite(float(snapshot["log_likelihood"])))
                self.assertTrue(np.isfinite(float(snapshot["bic"])))
                if "ari_vs_true_labels" in snapshot:
                    self.assertGreaterEqual(
                        float(snapshot["ari_vs_true_labels"]),
                        float(case.get("min_ari_to_true", 0.995)),
                    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--case", type=str, help="Case name in gmm_cases.json.")
    parser.add_argument(
        "--snapshot-stdout",
        action="store_true",
        help="Output sklearn snapshot JSON to stdout (no intermediate file).",
    )
    parser.add_argument(
        "--snapshot-auto-stdout",
        action="store_true",
        help="Output sklearn auto-selected snapshot JSON to stdout.",
    )
    parser.add_argument(
        "--criterion",
        type=str,
        default="bic",
        help="Selection criterion for --snapshot-auto-stdout: bic or aic.",
    )
    parser.add_argument("--min-components", type=int, help="Auto-selection minimum K.")
    parser.add_argument("--max-components", type=int, help="Auto-selection maximum K.")
    parser.add_argument(
        "--plot-stdin",
        action="store_true",
        help="Read Go/sklearn snapshots from stdin JSON and render comparison plot.",
    )
    args, remaining = parser.parse_known_args()

    if args.snapshot_stdout:
        if not args.case:
            raise ValueError("--snapshot-stdout requires --case")
        case = get_case_by_name(args.case)
        snapshot = fit_sklearn_snapshot(copy.deepcopy(case))
        json.dump(snapshot, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.snapshot_auto_stdout:
        if not args.case:
            raise ValueError("--snapshot-auto-stdout requires --case")
        case = get_case_by_name(args.case)
        snapshot = fit_sklearn_snapshot_auto(
            copy.deepcopy(case),
            criterion=args.criterion,
            min_components=args.min_components,
            max_components=args.max_components,
        )
        json.dump(snapshot, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.plot_stdin:
        payload = json.load(sys.stdin)
        result = render_comparison_plot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    unittest.main(argv=[sys.argv[0], *remaining])


if __name__ == "__main__":
    main()
