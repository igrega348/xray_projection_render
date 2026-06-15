#include <cmath>
#include <cuda_runtime.h>

#include "cuda_backend.h"

// Multiply a Vec3 by a 4x4 row-major matrix (ignoring homogeneous w)
__device__ void transform_point(
    const float m[16],
    float x, float y, float z,
    float& out_x, float& out_y, float& out_z)
{
    out_x = m[0] * x + m[1] * y + m[2] * z + m[3];
    out_y = m[4] * x + m[5] * y + m[6] * z + m[7];
    out_z = m[8] * x + m[9] * y + m[10] * z + m[11];
}

// Kernel for one camera; grid is (image_res x image_res) threads.
// Uses a 3D texture for hardware trilinear interpolation (normalized coords [0,1]).
__global__ void render_kernel(
    cudaTextureObject_t vol_tex,
    XRayCameraParams cam,
    int image_res,
    float ds,
    float flat_field,
    float cube_half_diagonal,
    float* __restrict__ out_image)
{
    int i = blockIdx.y * blockDim.y + threadIdx.y;
    int j = blockIdx.x * blockDim.x + threadIdx.x;
    if (i >= image_res || j >= image_res) {
        return;
    }

    float res_f = float(image_res);

    // Vertical field of view in radians
    float fov_rad = cam.fov_y * 3.1415926535f / 180.0f;
    float f = 1.0f / tanf(fov_rad * 0.5f);

    // Pixel coordinates in camera space, mirroring Go implementation
    float x_ndc = float(i) / (res_f * 0.5f) - 1.0f;
    float y_ndc = float(j) / (res_f * 0.5f) - 1.0f;
    float px = x_ndc;
    float py = y_ndc;
    float pz = -f;

    // Transform pixel position to world space
    float world_x, world_y, world_z;
    transform_point(cam.view, px, py, pz, world_x, world_y, world_z);

    float dir_x = world_x - cam.eye[0];
    float dir_y = world_y - cam.eye[1];
    float dir_z = world_z - cam.eye[2];

    // Normalize direction
    float len = sqrtf(dir_x * dir_x + dir_y * dir_y + dir_z * dir_z);
    if (len == 0.0f) {
        out_image[i * image_res + j] = 1.0f;
        return;
    }
    dir_x /= len;
    dir_y /= len;
    dir_z /= len;

    // Integration bounds (approximate the Go code's use of R ± cube_half_diagonal)
    float smin = cam.R - cube_half_diagonal;
    float smax = cam.R + cube_half_diagonal;

    float T = flat_field;
    for (float s = smin; s < smax; s += ds) {
        float x = cam.eye[0] + dir_x * s;
        float y = cam.eye[1] + dir_y * s;
        float z = cam.eye[2] + dir_z * s;
        // Normalized coords [0,1]; array layout is (ny, nx, nz) to match host
        float rho = tex3D<float>(vol_tex, (y + 1.0f) * 0.5f, (x + 1.0f) * 0.5f, (z + 1.0f) * 0.5f);
        T += rho * ds;
    }

    out_image[i * image_res + j] = expf(-T);
}

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
)
{
    if (!volume || !cameras || !out_images) {
        return 1;
    }
    if (nx <= 0 || ny <= 0 || nz <= 0 || image_res <= 0 || num_cameras <= 0) {
        return 2;
    }

    // 3D array layout (ny, nx, nz) to match host: idx = k*nx*ny + i*ny + j
    cudaChannelFormatDesc channelDesc = cudaCreateChannelDesc<float>();
    cudaExtent extent;
    extent.width = ny;
    extent.height = nx;
    extent.depth = nz;

    cudaArray* d_vol_array = nullptr;
    cudaError_t err = cudaMalloc3DArray(&d_vol_array, &channelDesc, extent);
    if (err != cudaSuccess) {
        return 3;
    }

    cudaMemcpy3DParms copyParams = {};
    copyParams.srcPtr.ptr = const_cast<float*>(volume);
    copyParams.srcPtr.pitch = static_cast<size_t>(ny) * sizeof(float);
    copyParams.srcPtr.xsize = static_cast<size_t>(ny) * sizeof(float);
    copyParams.srcPtr.ysize = nx;
    copyParams.dstArray = d_vol_array;
    copyParams.extent = extent;
    copyParams.kind = cudaMemcpyHostToDevice;

    err = cudaMemcpy3D(&copyParams);
    if (err != cudaSuccess) {
        cudaFreeArray(d_vol_array);
        return 4;
    }

    cudaResourceDesc resDesc = {};
    resDesc.resType = cudaResourceTypeArray;
    resDesc.res.array.array = d_vol_array;

    cudaTextureDesc texDesc = {};
    texDesc.addressMode[0] = cudaAddressModeClamp;
    texDesc.addressMode[1] = cudaAddressModeClamp;
    texDesc.addressMode[2] = cudaAddressModeClamp;
    texDesc.filterMode = cudaFilterModeLinear;
    texDesc.readMode = cudaReadModeElementType;
    texDesc.normalizedCoords = 1;

    cudaTextureObject_t vol_tex = 0;
    err = cudaCreateTextureObject(&vol_tex, &resDesc, &texDesc, nullptr);
    if (err != cudaSuccess) {
        cudaFreeArray(d_vol_array);
        return 5;
    }

    size_t image_pixels = static_cast<size_t>(image_res) * image_res;
    size_t image_bytes = image_pixels * sizeof(float);

    float* d_image = nullptr;
    err = cudaMalloc(&d_image, image_bytes);
    if (err != cudaSuccess) {
        cudaDestroyTextureObject(vol_tex);
        cudaFreeArray(d_vol_array);
        return 6;
    }

    dim3 block(16, 16);
    dim3 grid(
        (image_res + block.x - 1) / block.x,
        (image_res + block.y - 1) / block.y
    );

    const float cube_half_diagonal = 1.74f;

    for (int c = 0; c < num_cameras; ++c) {
        const XRayCameraParams& cam = cameras[c];

        render_kernel<<<grid, block>>>(
            vol_tex,
            cam,
            image_res,
            ds,
            flat_field,
            cube_half_diagonal,
            d_image
        );

        err = cudaDeviceSynchronize();
        if (err != cudaSuccess) {
            cudaFree(d_image);
            cudaDestroyTextureObject(vol_tex);
            cudaFreeArray(d_vol_array);
            return 7;
        }

        float* out_image = out_images + c * image_pixels;
        err = cudaMemcpy(out_image, d_image, image_bytes, cudaMemcpyDeviceToHost);
        if (err != cudaSuccess) {
            cudaFree(d_image);
            cudaDestroyTextureObject(vol_tex);
            cudaFreeArray(d_vol_array);
            return 8;
        }
    }

    cudaFree(d_image);
    cudaDestroyTextureObject(vol_tex);
    cudaFreeArray(d_vol_array);
    return 0;
}

// Each thread evaluates one voxel against all cylinders.
// Volume layout: out[k*res*res + i*res + j]
// World coords:  x = i/res*2-1, y = j/res*2-1, z = k/res*2-1
__global__ void voxelize_kernel(
    const CylinderParams* __restrict__ cylinders,
    int num_cylinders,
    int res,
    float density_multiplier,
    float* __restrict__ out_volume)
{
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    int total = res * res * res;
    if (idx >= total) return;

    int k = idx / (res * res);
    int i = (idx / res) % res;
    int j = idx % res;

    float res_f = (float)res;
    float x = (float)i / res_f * 2.0f - 1.0f;
    float y = (float)j / res_f * 2.0f - 1.0f;
    float z = (float)k / res_f * 2.0f - 1.0f;

    float density = 0.0f;
    for (int c = 0; c < num_cylinders; c++) {
        float vx = cylinders[c].p1[0] - cylinders[c].p0[0];
        float vy = cylinders[c].p1[1] - cylinders[c].p0[1];
        float vz = cylinders[c].p1[2] - cylinders[c].p0[2];
        float wx = x - cylinders[c].p0[0];
        float wy = y - cylinders[c].p0[1];
        float wz = z - cylinders[c].p0[2];
        float vdotv = vx*vx + vy*vy + vz*vz;
        if (vdotv == 0.0f) continue;
        float t = (wx*vx + wy*vy + wz*vz) / vdotv;
        if (t < 0.0f || t > 1.0f) continue;
        float dx = wx - vx*t;
        float dy = wy - vy*t;
        float dz = wz - vz*t;
        float r = cylinders[c].radius;
        if (dx*dx + dy*dy + dz*dz < r*r) {
            density += cylinders[c].rho;
        }
    }
    density *= density_multiplier;
    if (density > 1.0f) density = 1.0f;
    if (density < 0.0f) density = 0.0f;
    out_volume[idx] = density;
}

int AssembleVoxelGridCUDA(
    const CylinderParams* cylinders,
    int num_cylinders,
    int res,
    float density_multiplier,
    float* out_volume)
{
    if (!cylinders || !out_volume || num_cylinders <= 0 || res <= 0) return 1;

    size_t cyl_bytes = (size_t)num_cylinders * sizeof(CylinderParams);
    size_t vol_bytes = (size_t)res * res * res * sizeof(float);

    CylinderParams* d_cyls = nullptr;
    float* d_vol = nullptr;
    cudaError_t err;

    err = cudaMalloc(&d_cyls, cyl_bytes);
    if (err != cudaSuccess) return 2;

    err = cudaMemcpy(d_cyls, cylinders, cyl_bytes, cudaMemcpyHostToDevice);
    if (err != cudaSuccess) { cudaFree(d_cyls); return 3; }

    err = cudaMalloc(&d_vol, vol_bytes);
    if (err != cudaSuccess) { cudaFree(d_cyls); return 4; }

    int total = res * res * res;
    int block = 256;
    int grid = (total + block - 1) / block;

    voxelize_kernel<<<grid, block>>>(d_cyls, num_cylinders, res, density_multiplier, d_vol);

    err = cudaDeviceSynchronize();
    if (err != cudaSuccess) { cudaFree(d_cyls); cudaFree(d_vol); return 5; }

    err = cudaMemcpy(out_volume, d_vol, vol_bytes, cudaMemcpyDeviceToHost);
    cudaFree(d_cyls);
    cudaFree(d_vol);
    return (err != cudaSuccess) ? 6 : 0;
}

// Spatial-hash variant: each voxel only checks the cylinders assigned to its grid cell.
// cell_offsets[c] and cell_offsets[c+1] delimit the slice of cyl_indices for cell c.
// Grid ordering: cell = (cz * grid_dim + cy) * grid_dim + cx
__global__ void voxelize_spatial_kernel(
    const CylinderParams* __restrict__ cylinders,
    int res,
    float density_multiplier,
    int grid_dim,
    const int* __restrict__ cell_offsets,
    const int* __restrict__ cyl_indices,
    float* __restrict__ out_volume)
{
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    int total = res * res * res;
    if (idx >= total) return;

    int k = idx / (res * res);
    int i = (idx / res) % res;
    int j = idx % res;

    float res_f = (float)res;
    float x = (float)i / res_f * 2.0f - 1.0f;
    float y = (float)j / res_f * 2.0f - 1.0f;
    float z = (float)k / res_f * 2.0f - 1.0f;

    // Map world coords to grid cell.
    float cell_size = 2.0f / (float)grid_dim;
    int cx = (int)((x + 1.0f) / cell_size);
    int cy = (int)((y + 1.0f) / cell_size);
    int cz = (int)((z + 1.0f) / cell_size);
    if (cx < 0) cx = 0; if (cx >= grid_dim) cx = grid_dim - 1;
    if (cy < 0) cy = 0; if (cy >= grid_dim) cy = grid_dim - 1;
    if (cz < 0) cz = 0; if (cz >= grid_dim) cz = grid_dim - 1;
    int cell = (cz * grid_dim + cy) * grid_dim + cx;

    int start = cell_offsets[cell];
    int end   = cell_offsets[cell + 1];

    float density = 0.0f;
    for (int ci = start; ci < end; ci++) {
        int c = cyl_indices[ci];
        float vx = cylinders[c].p1[0] - cylinders[c].p0[0];
        float vy = cylinders[c].p1[1] - cylinders[c].p0[1];
        float vz = cylinders[c].p1[2] - cylinders[c].p0[2];
        float wx = x - cylinders[c].p0[0];
        float wy = y - cylinders[c].p0[1];
        float wz = z - cylinders[c].p0[2];
        float vdotv = vx*vx + vy*vy + vz*vz;
        if (vdotv == 0.0f) continue;
        float t = (wx*vx + wy*vy + wz*vz) / vdotv;
        if (t < 0.0f || t > 1.0f) continue;
        float dx = wx - vx*t;
        float dy = wy - vy*t;
        float dz = wz - vz*t;
        float r = cylinders[c].radius;
        if (dx*dx + dy*dy + dz*dz < r*r) {
            density += cylinders[c].rho;
        }
    }
    density *= density_multiplier;
    if (density > 1.0f) density = 1.0f;
    if (density < 0.0f) density = 0.0f;
    out_volume[idx] = density;
}

int AssembleVoxelGridSpatialCUDA(
    const CylinderParams* cylinders,
    int num_cylinders,
    int res,
    float density_multiplier,
    int grid_dim,
    const int* cell_offsets,
    const int* cyl_indices,
    int num_cyl_indices,
    float* out_volume)
{
    if (!cylinders || !out_volume || !cell_offsets || !cyl_indices) return 1;
    if (num_cylinders <= 0 || res <= 0 || grid_dim <= 0) return 2;

    size_t cyl_bytes     = (size_t)num_cylinders   * sizeof(CylinderParams);
    size_t vol_bytes     = (size_t)res * res * res  * sizeof(float);
    size_t off_bytes     = (size_t)(grid_dim * grid_dim * grid_dim + 1) * sizeof(int);
    size_t idx_bytes     = (size_t)num_cyl_indices  * sizeof(int);

    CylinderParams* d_cyls = nullptr;
    float*          d_vol  = nullptr;
    int*            d_off  = nullptr;
    int*            d_idx  = nullptr;
    cudaError_t err;

#define CHECKED_ALLOC(ptr, bytes) \
    err = cudaMalloc(&ptr, bytes); \
    if (err != cudaSuccess) { cudaFree(d_cyls); cudaFree(d_vol); cudaFree(d_off); cudaFree(d_idx); return 3; }

    CHECKED_ALLOC(d_cyls, cyl_bytes)
    CHECKED_ALLOC(d_vol,  vol_bytes)
    CHECKED_ALLOC(d_off,  off_bytes)
    CHECKED_ALLOC(d_idx,  idx_bytes)
#undef CHECKED_ALLOC

#define CHECKED_COPY(dst, src, bytes, kind) \
    err = cudaMemcpy(dst, src, bytes, kind); \
    if (err != cudaSuccess) { cudaFree(d_cyls); cudaFree(d_vol); cudaFree(d_off); cudaFree(d_idx); return 4; }

    CHECKED_COPY(d_cyls, cylinders,    cyl_bytes, cudaMemcpyHostToDevice)
    CHECKED_COPY(d_off,  cell_offsets, off_bytes, cudaMemcpyHostToDevice)
    CHECKED_COPY(d_idx,  cyl_indices,  idx_bytes, cudaMemcpyHostToDevice)
#undef CHECKED_COPY

    int total = res * res * res;
    int block  = 256;
    int grid   = (total + block - 1) / block;

    voxelize_spatial_kernel<<<grid, block>>>(
        d_cyls, res, density_multiplier,
        grid_dim, d_off, d_idx, d_vol);

    err = cudaDeviceSynchronize();
    if (err != cudaSuccess) { cudaFree(d_cyls); cudaFree(d_vol); cudaFree(d_off); cudaFree(d_idx); return 5; }

    err = cudaMemcpy(out_volume, d_vol, vol_bytes, cudaMemcpyDeviceToHost);
    cudaFree(d_cyls); cudaFree(d_vol); cudaFree(d_off); cudaFree(d_idx);
    return (err != cudaSuccess) ? 6 : 0;
}

