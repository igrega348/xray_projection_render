// CUDA volume rendering backend for xray_projection_render.
//
// This header defines a narrow C API that can be called from Go (via cgo)
// and potentially other languages. The implementation is expected to live
// in a CUDA-compiled translation unit (e.g. cuda_backend.cu).
//
// The design assumes a volume-based rendering pipeline:
//   - The scene is pre-discretized into a regular 3D grid of densities
//     in the normalized coordinate range [-1, 1]^3.
//   - The CUDA kernel performs Beer-Lambert integration along rays for
//     one or more camera views, mirroring the semantics of the existing
//     Go implementation (fixed step size ds, flat_field offset).

#ifndef XRAY_PROJECTION_RENDER_CUDA_BACKEND_H
#define XRAY_PROJECTION_RENDER_CUDA_BACKEND_H

#ifdef __cplusplus
extern "C" {
#endif

// Parameters for a single cylinder primitive.
typedef struct {
    float p0[3];    // start endpoint
    float p1[3];    // end endpoint
    float radius;
    float rho;      // density value when inside
} CylinderParams;

// Assemble a float32 voxel grid by evaluating cylinder densities on the GPU.
// Brute-force: each voxel checks all num_cylinders.
//
// Returns: 0 on success, non-zero on error.
int AssembleVoxelGridCUDA(
    const CylinderParams* cylinders,
    int num_cylinders,
    int res,
    float density_multiplier,
    float* out_volume
);

// Spatial-hash accelerated variant.
// The caller pre-builds a CSR (compressed sparse row) structure that maps each
// grid cell to its list of candidate cylinder indices.
//
// Arguments:
//   cylinders         - cylinder params, length num_cylinders
//   num_cylinders     - total cylinders
//   res               - cubic volume side length
//   density_multiplier
//   grid_dim          - spatial hash grid side length (G; total G³ cells)
//   cell_offsets      - int array of length G³+1; cell_offsets[c] is the start
//                       index into cyl_indices for cell c, cell_offsets[G³] == len(cyl_indices)
//   cyl_indices       - int array of cylinder indices, one entry per (cell, cylinder) pair
//   num_cyl_indices   - length of cyl_indices
//   out_volume        - output, length res³, layout [k*res*res + i*res + j]
//
// Returns: 0 on success, non-zero on error.
int AssembleVoxelGridSpatialCUDA(
    const CylinderParams* cylinders,
    int num_cylinders,
    int res,
    float density_multiplier,
    int grid_dim,
    const int* cell_offsets,
    const int* cyl_indices,
    int num_cyl_indices,
    float* out_volume
);

// Camera parameters for one projection.
//
// The camera transform follows the convention used in the Go code:
//   - "view" is the 4x4 matrix returned by computeCameraFromAngles, stored
//     in row-major order (view[row * 4 + col]).
//   - "eye" is the camera position in world coordinates.
//   - "fov_y" is the vertical field of view in degrees.
//   - "R" is the distance from the camera to the scene center.
typedef struct {
    float eye[3];
    float view[16];
    float fov_y;
    float R;
} XRayCameraParams;

// Render a batch of projections from a precomputed volume.
//
// Arguments:
//   volume        - pointer to volume data, length nx * ny * nz
//   nx, ny, nz    - volume dimensions; layout is z-major (k), then x (i), then y (j):
//                   idx = k * nx * ny + i * ny + j
//   cameras       - array of camera parameters of length num_cameras
//   num_cameras   - number of cameras / projections to render
//   image_res     - resolution of square output images (image_res x image_res)
//   ds            - integration step size along the ray
//   flat_field    - flat field term added to the optical thickness before exp()
//   out_images    - output buffer of length num_cameras * image_res * image_res
//                   layout: images[cam * image_res * image_res + i * image_res + j]
//
// Returns:
//   0 on success, non-zero on error.
int RenderVolumeProjectionsCUDA(
    const float* volume,
    int nx,
    int ny,
    int nz,
    const XRayCameraParams* cameras,
    int num_cameras,
    int image_res,
    float ds,
    float flat_field,
    float* out_images
);

#ifdef __cplusplus
} // extern "C"
#endif

#endif // XRAY_PROJECTION_RENDER_CUDA_BACKEND_H

