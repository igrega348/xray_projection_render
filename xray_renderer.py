"""
Python bindings for xray_projection_render using ctypes.

This module provides a Python interface to the xray_projection_render Go library.
The library must be built as a shared library first using build.sh.

Example usage:
    from xray_renderer import XRayRenderer
    
    # Initialize the renderer
    renderer = XRayRenderer()
    
    # Set up parameters
    params = {
        'input': 'examples/cube_w_hole.yaml',
        'output_dir': 'images',
        'resolution': 512,
        'camera_angles': [
            {'azimuthal': 0, 'polar': 90},
            {'azimuthal': 45, 'polar': 90},
            {'azimuthal': 90, 'polar': 90},
        ],
        'R': 4.0,
        'fov': 40.0,
    }
    
    # Render
    result = renderer.render(params)
    print(result)
"""

import ctypes
import json
import os
import platform
import sys
from pathlib import Path
from typing import Dict, List, Optional, Union


class XRayRenderer:
    """Python wrapper for the xray_projection_render Go library."""
    
    def __init__(self, library_path: Optional[str] = None):
        """
        Initialize the renderer by loading the shared library.
        
        Args:
            library_path: Path to the shared library file. If None, attempts to
                         find it in common locations relative to this file.
        """
        if library_path is None:
            library_path = self._find_library()
        
        if not os.path.exists(library_path):
            raise FileNotFoundError(
                f"Library not found at {library_path}. "
                "Please build the shared library first using build.sh"
            )
        
        self.lib = ctypes.CDLL(library_path)
        self._setup_function_signatures()
    
    def _find_library(self) -> str:
        """Find the shared library file based on the current platform."""
        # Get the directory where this file is located
        script_dir = Path(__file__).parent.resolve()
        build_dir = script_dir / "build"
        
        system = platform.system().lower()
        machine = platform.machine().lower()
        
        if system == "darwin":
            # On macOS, Go may create the library without .dylib extension
            lib_names = ["libxray_projection_render.dylib", "libxray_projection_render"]
        elif system == "windows":
            lib_names = ["xray_projection_render.dll"]
        else:  # Linux and others
            lib_names = ["libxray_projection_render.so"]
        
        # Try each possible library name
        for lib_name in lib_names:
            lib_path = build_dir / lib_name
            if lib_path.exists():
                return str(lib_path)
            
            # Fallback: try current directory
            lib_path = script_dir / lib_name
            if lib_path.exists():
                return str(lib_path)
        
        # Return expected path for error message
        return str(build_dir / lib_names[0])
    
    def _setup_function_signatures(self):
        """Set up the function signatures for ctypes."""
        # RenderProjections: takes a C string, returns a C string
        self.lib.RenderProjections.argtypes = [ctypes.c_char_p]
        self.lib.RenderProjections.restype = ctypes.POINTER(ctypes.c_char)
        
        # FreeString: frees a C string
        self.lib.FreeString.argtypes = [ctypes.c_char_p]
        self.lib.FreeString.restype = None
    
    def render(
        self,
        params: Dict,
        camera_angles: Optional[List[Dict[str, float]]] = None
    ) -> Dict:
        """
        Render X-ray projections based on the provided parameters.
        
        Args:
            params: Dictionary containing render parameters. Supported keys:
                - input: Path to input YAML/JSON file describing the object (required)
                - output_dir: Output directory for images (default: "images")
                - fname_pattern: Filename pattern with sprintf format (default: "image_%03d.png")
                - resolution: Image resolution (default: 512)
                - num_images: Number of images for equispaced angle generation (default: 1)
                - out_of_plane: Use random polar angles (default: False)
                - ds: Integration step size, negative to auto-compute (default: -1.0)
                - R: Distance from camera to scene center (default: 4.0)
                - fov: Field of view in degrees (default: 40.0)
                - jobs_modulo: Job modulo for parallel execution (default: 1)
                - job_num: Job number for parallel execution (default: 0)
                - transforms_file: Output file for transform parameters (default: "transforms.json")
                - deformation_file: Path to deformation file (default: "")
                - time_label: Time label for metadata (default: 0.0)
                - transparency: Enable transparency in output (default: False)
                - export_volume: Export volume grid (default: False)
                - polar_angle: Fixed polar angle in degrees (default: 90.0)
                - density_multiplier: Density multiplier (default: 1.0)
                - flat_field: Flat field value (default: 0.0)
                - integration: Integration method "simple" or "hierarchical" (default: "hierarchical")
            camera_angles: Optional list of camera angle dictionaries with 'azimuthal' and 'polar' keys.
                          If provided, overrides num_images/out_of_plane/polar_angle parameters.
        
        Returns:
            Dictionary with render results:
                - success: Boolean indicating success
                - error: Error message if failed
                - num_images: Number of images rendered
                - output_dir: Output directory path
        """
        # Prepare parameters dict
        render_params = {
            "input": params.get("input"),
            "output_dir": params.get("output_dir", "images"),
            "fname_pattern": params.get("fname_pattern", "image_%03d.png"),
            "resolution": params.get("resolution", 512),
            "num_images": params.get("num_images", 1),
            "out_of_plane": params.get("out_of_plane", False),
            "ds": params.get("ds", -1.0),
            "R": params.get("R", 4.0),
            "fov": params.get("fov", 40.0),
            "jobs_modulo": params.get("jobs_modulo", 1),
            "job_num": params.get("job_num", 0),
            "transforms_file": params.get("transforms_file", "transforms.json"),
            "deformation_file": params.get("deformation_file", ""),
            "time_label": params.get("time_label", 0.0),
            "transparency": params.get("transparency", False),
            "export_volume": params.get("export_volume", False),
            "polar_angle": params.get("polar_angle", 90.0),
            "density_multiplier": params.get("density_multiplier", 1.0),
            "flat_field": params.get("flat_field", 0.0),
            "integration": params.get("integration", "hierarchical"),
            "camera_angles": [],
        }
        
        # Handle camera_angles parameter
        if camera_angles is not None:
            render_params["camera_angles"] = camera_angles
        elif "camera_angles" in params:
            render_params["camera_angles"] = params["camera_angles"]
        
        # Validate required parameters
        if not render_params["input"]:
            raise ValueError("'input' parameter is required")
        
        # Convert to JSON string
        params_json = json.dumps(render_params)
        params_bytes = params_json.encode('utf-8')
        
        # Call the C function
        result_ptr = self.lib.RenderProjections(params_bytes)
        
        # Convert result back to Python string
        result_str = ctypes.string_at(result_ptr).decode('utf-8')
        
        # Free the C string
        self.lib.FreeString(result_ptr)
        
        # Parse and return result
        return json.loads(result_str)

