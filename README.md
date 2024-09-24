# X-ray projection engine

Go routines to generate x-ray projections of simple objects.

The projections are saved as images and corresponding `transforms.json` file is produced which can be used in NeRF framework.

The framework enables for the compositions of simple objects (e.g. sphere, cylinder, box, cube, parallelepiped) as well as periodic tessellations.

In the image is an object created as a composition of a cube $(\rho=1)$, a sphere $(\rho=-1)$, and a cylinder $(\rho=-1)$. It can be generated by running either of the following:
```
# running source code
go run . --input examples/cube_w_hole.yaml
# executable on Windows
xray_projection_render.exe --input examples/cube_w_hole.yaml
# executable on Linux
./xray_projection_render --input examples/cube_w_hole.yaml
```

![Cube with hole](https://github.com/igrega348/xray_projection_render/blob/main/examples/cube_00.png?raw=true)

To see all available options, run

```
./xray_projection_render -h
```

## Highlighted features

### Accepted input formats and object library

Both standard `json` and `yaml` files are accepted as input descriptors of objects. See examples provided in the `examples` folder.
The supported objects have the following attributes

| Object type                         | Attibutes |
|--------------------------------|-----------|
| sphere         | center[3], radius, rho, type=sphere |
| cylinder      | p0[3], p1[3], radius, rho, type=cylinder |
| cube          | center[3], side, rho, type=cube |
| box           | center[3], sides[3], rho, type=box |
| parallelepiped | origin[3], v0[3], v1[3], v2[3], rho, type=parallelepiped |
| object_collection | objects[*], type=object_collection |
| unit_cell     | objects::object_collection, xmin, xmax, ymin, ymax, zmin, zmax, type=unit_cell |
| tessellated_obj_coll | uc::unit_cell, xmin, xmax, ymin, ymax, zmin, zmax, type=tessellated_obj_coll |

Where appropriate, attribute types are denoted by `::`, and corresponding array length by square brackets `[]`.
All object types can be found in the examples provided in the `examples` directory.

### Simulate tomographic X-ray projections

Given an object file (e.g. `examples/cube_w_hole.yaml`), a simple set of X-ray projections can be generated by running the following

```
go run . --input examples/cube_w_hole.yaml --num_projections 16
```

This will generate 16 equispaced projections in the horizontal plane, as well as `transforms.json` file describing the locations of the corresponding cameras. 
This file is readable using standard NeRF packages.

Many options can be used to control the output. A few examples are:
`--resolution`, `--density_multiplier`, `--text_progress`, `-v`.
Two key parameters which specify the field of view and distance of the equivalent camera are `--fov` and `-R`.
Setting `--out_of_plane` will generate a projection at random elevation angles. While this is not typical for X-ray computed tomography, it can be useful as a test set for the evaluation of NeRF reconstruction.

### Hierarchical integration (ray tracing)

The attenuation equation for X-ray comes from the _Beer-Lambert Law_:

$$
I = I_0 \exp\left(-\int_{s_0}^{s_1} \rho(s) \mathrm{ds} \right)
$$

where $I_0$ is the intensity of the empty image and $I$ is the attenuated intensity recorded at the detector/camera.
The ray is assumed to go between $s_0$ and $s_1$.

Two functions are implemented for integration along the ray: _integrate_hierarchical_ (default) and _integrate_along_ray_.
Both are controlled by parameter `--ds`.

_integrate_along_ray_ is simple numerical integration where the ray is divided into equal segments of length $ds$ and the integral is approximated as the sum $\int \rho(s) \mathrm{ds} \approx \sum \rho(s) ds$

_integrate_hierarchical_ is a little bit more advanced. The motivation is to avoid banding artifacts without uniformly reducing $ds$. It works by only refining $ds$ in segments in which the density changed between the left and right boundary.

Normally, `ds` is calculated automatically from the dimensions on the objects. Its value is displayed to the output if `-v` (verbose) flag is active and it may need to be reduced if banding artifacts are observed.

### Deformations

It is possible to apply topology-preserving deformations to the object by using `--deformation_file` with an appropriate deformation descriptor file (yaml).
For example, one can run

```
go run . --input examples/cube_w_hole.yaml --deformation_file examples/deformation.yaml
```

When querying density field, coordinates will be remapped using the chosen deformation field.
Implemented deformations are _Sigmoid_, _Rigid_, _Linear_, _Gaussian_ with the following deformation fields:
- Rigid: parametrized by three-component constant displacement vector $[u_x,u_y,u_z]$.
  
$$
x\leftarrow x+u_x; \, y\leftarrow y+u_y; \, z\leftarrow z+u_z
$$

- Linear: parametrized by 3 strains $[\epsilon_x, \epsilon_y, \epsilon_z]$.

$$
x \leftarrow x (1+ \epsilon_x); \, y \leftarrow y (1+ \epsilon_y); \, z \leftarrow z (1+ \epsilon_z)
$$

- Sigmoid: parametrized by _amplitude A, center c, lengthscale L, direction_. For direction 'z':

$$
x \leftarrow x; \, y \leftarrow y; \, z \leftarrow z+\frac{A}{1+\exp\left( - (z-c)/L \right) }
$$

- Gaussian: parametrized by _amplitudes A[3], sigmas s[3], centers c[3]_.

$$
r \leftarrow \sqrt{(x-c[0])^2+(y-c[1])^2+(z-c[2])^2}
$$

$$
x \leftarrow x+A[0]\exp\left(-\frac{r^2}{2s[0]^2}\right); \, y \leftarrow y+A[1]\exp\left(-\frac{r^2}{2s[1]^2}\right); \mathrm{etc.}
$$

### Splitting of long jobs

While projections of simple objects are generated within seconds, for some complex volumes (e.g. fully resolved lattices) at high resolutions, it can take several minutes to generate each projection.
For these reasons, the program can be run in parallel, with each instance only rendering projections based on modulo arithmetic.

For example, suppose we wish to generate 256 projections as 8 parallel jobs. This can be done by running 8 commands independently

```
go run . --input object.yaml --jobs_modulo 4 --job [x] --transforms_file transforms_[x].json
```
where `[x]` goes from 0 to 7.
The images will be saved independently, and all the transforms file can be combined in a post-processing step.

Option `--text_progress` can be set for purely text-based indication of render progress for each image, instead of the deafult progress bar for the whole job; can be useful when running on servers.
