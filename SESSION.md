# Session Pack
**Packed:** 2026-06-14
**Project:** xray_projection_render
**Session goal:** Adversarial review of CUDA backend and full renderer for scientific validity and external API safety.

## Status
🔄 In progress — review complete, no fixes applied yet

## Completed
- Full adversarial review of the CUDA backend (`cuda_backend.cu`, `cuda_backend.go`, `cuda_backend.h`, `cuda_path.go`, `cuda_voxel.go`)
- Full adversarial review of the CPU renderer, Python API, and code layout (`main.go`, `api.go`, `objects/`, `deformations/`, `main_test.go`)
- Confirmed pixel coordinate orientation is **not** a bug — X-ray transmission images are naturally mirrored relative to camera convention; user verified with asymmetric object

## In Progress / Last Action
Two `/adversarial-review` runs completed. No code changes made this session. The session ended with reviewing findings and the user correcting one false positive (pixel transpose).

## Next Step
Address the highest-priority bugs found in the review. Recommended order:

1. **Fix `log.Fatal().Err(err)` missing `.Msg()` calls** (silent error drops) — at least 4 sites in `main.go` (lines ~59, 95, 513, 529). Change to `log.Fatal().Err(err).Msg("description")`.

2. **Fix `VoxelGrid` index layout mismatch** between `Density()` (`[z][y][x]`) and `ExportToRaw` (`[z][x][y]`) — round-trip through export/import silently swaps x and y axes.

3. **Fix hierarchical integrator double-count** at material boundaries (`main.go:~175`) — the `T += rho * ds` after the inner refinement loop adds the transition point twice.

4. **Write the sphere chord-length test** (highest-value scientific ground-truth test):
   ```go
   // Sphere of radius r=0.5, density rho=1.0 at origin
   // Ray through center should give exp(-2*r*rho) = exp(-1.0)
   ```

## Failed / Blocked
No code changes attempted — review only. No build or test failures.

## Open Questions / Risks

### Scientific validity gaps (no tests exist for any of these)
| Gap | Risk |
|---|---|
| CPU vs CUDA pixel value agreement | CUDA path could be systematically wrong; completely undetected |
| Hierarchical vs simple integrator agreement | Double-count bug goes undetected |
| Sphere/slab analytical ground-truth | No quantitative correctness check anywhere |
| VoxelGrid round-trip (export → import → Density) | Index swap bug undetected |
| `TessellatedObjColl` continuity at unit-cell boundaries | Zero-density seams in lattice projections |
| Negative-rho (carved-out) objects with `GreedyDensEval=true` | Holes silently not carved when tessellated |

### Other open risks
- `AffineDeformation` replaces coordinates (pure linear map) rather than adding displacement — inconsistent with all other deformation types; undocumented
- `ds` step size for CUDA path (`cuda_path.go:46`) based on `NX` only — wrong for non-cubic volumes
- Texture x/y axis permutation in CUDA kernel silently wrong for non-cubic volumes (`cuda_backend.cu:75`)
- CUDA render path silently ignores deformation files with no warning
- CUDA inaccessible from Python API (`api.go:139` hardcodes `use_cuda: false`)
- `RenderProjections` not thread-safe (global mutable state)
- `log.Fatal` in error paths calls `os.Exit` — the `recover()` in `RenderProjections` cannot catch it
- No API versioning — missing JSON fields silently zero-initialize (e.g. `density_multiplier: 0` renders nothing)
- `flat_field` unit inconsistency: integrated as optical depth but stored as `exp(-flat_field)` in JSON output

## Environment
- Branch: `cuda-backend-wired`
- Go module: see `go.mod`
- CUDA binary: `xray_render_cuda` (built, exists at repo root)
- Shared library: `libcuda_render.so` (built, exists at repo root)
- Python package installed in editable mode (see `pyproject.toml`)
- Studio: Lightning AI GPU studio (Linux, CUDA available)

## Key Paths
| Artifact | Path | Exists |
|---|---|---|
| Main renderer | `main.go` | ✅ |
| C/Python API | `api.go` | ✅ |
| CUDA kernel | `cuda_backend.cu` | ✅ |
| CUDA Go glue | `cuda_backend.go` | ✅ |
| CUDA path render | `cuda_path.go` | ✅ |
| CUDA voxelizer | `cuda_voxel.go` | ✅ |
| Test file (sparse) | `main_test.go` | ✅ |
| CUDA binary | `xray_render_cuda` | ✅ |
| Shared library | `libcuda_render.so` | ✅ |
| Python wrapper | `xray_projection_render/xray_renderer.py` | ✅ |
| Objects package | `objects/` | ✅ |
| Deformations package | `deformations/` | ✅ |
| Memory: pixel orientation note | `.claude/projects/.../memory/feedback_xray_pixel_orientation.md` | ✅ |
