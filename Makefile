CUDA_HOME ?= /usr/local/cuda
NVCC      ?= $(CUDA_HOME)/bin/nvcc

# sm_75..sm_90: covers Turing through Hopper; compiles with CUDA 11.8+
GENCODE_CUDA12 = \
    -gencode arch=compute_75,code=sm_75 \
    -gencode arch=compute_80,code=sm_80 \
    -gencode arch=compute_86,code=sm_86 \
    -gencode arch=compute_89,code=sm_89 \
    -gencode arch=compute_90,code=sm_90 \
    -gencode arch=compute_90,code=compute_90

# adds sm_100 (Blackwell GB100) and sm_120 (GB200); requires CUDA 12.8+ / 13+
GENCODE_CUDA13 = $(GENCODE_CUDA12) \
    -gencode arch=compute_100,code=sm_100 \
    -gencode arch=compute_120,code=sm_120 \
    -gencode arch=compute_120,code=compute_120

NVCC_FLAGS = -O2 -shared -Xcompiler -fPIC \
    -I$(CUDA_HOME)/include \
    -L$(CUDA_HOME)/lib64 -lcudart

libcuda_render.so: cuda_backend.cu cuda_backend.h
	$(NVCC) $(NVCC_FLAGS) $(GENCODE_CUDA12) -o $@ $<

libcuda_render-cuda13.so: cuda_backend.cu cuda_backend.h
	$(NVCC) $(NVCC_FLAGS) $(GENCODE_CUDA13) -o $@ $<

clean:
	rm -f libcuda_render.so libcuda_render-cuda13.so

.PHONY: clean
