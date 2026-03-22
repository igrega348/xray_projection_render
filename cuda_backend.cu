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

