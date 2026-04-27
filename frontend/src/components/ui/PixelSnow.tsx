import React, { useEffect, useRef } from 'react';
import * as THREE from 'three';
import './PixelSnow.css';

interface PixelSnowProps {
  color?: string;
  flakeSize?: number;
  minFlakeSize?: number;
  pixelResolution?: number;
  speed?: number;
  density?: number;
  direction?: number;
  brightness?: number;
  variant?: 'square' | 'round' | 'diamond';
  className?: string;
  style?: React.CSSProperties;
}

export const PixelSnow: React.FC<PixelSnowProps> = ({
  color = '#ffffff',
  flakeSize = 0.01,
  minFlakeSize = 0.5,
  pixelResolution = 128,
  speed = 1.0,
  density = 0.2,
  direction = 180,
  brightness = 1.0,
  variant = 'square',
  className = '',
  style = {},
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<THREE.Scene | null>(null);
  const cameraRef = useRef<THREE.OrthographicCamera | null>(null);
  const rendererRef = useRef<THREE.WebGLRenderer | null>(null);
  const flakesRef = useRef<THREE.Points | null>(null);
  const animationFrameRef = useRef<number | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Scene setup
    const scene = new THREE.Scene();
    sceneRef.current = scene;

    // Camera setup
    const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 10);
    camera.position.z = 1;
    cameraRef.current = camera;

    // Renderer setup
    const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: false });
    renderer.setSize(containerRef.current.clientWidth, containerRef.current.clientHeight);
    renderer.setPixelRatio(1);
    containerRef.current.appendChild(renderer.domElement);
    rendererRef.current = renderer;

    // Calculate number of flakes based on density
    const flakeCount = Math.floor(density * 1000);

    // Create flake geometry
    const geometry = new THREE.BufferGeometry();
    const positions = new Float32Array(flakeCount * 3);
    const sizes = new Float32Array(flakeCount);
    const velocities = new Float32Array(flakeCount * 2);

    const directionRad = (direction * Math.PI) / 180;
    const windX = Math.sin(directionRad) * speed * 0.1;
    const windY = -Math.cos(directionRad) * speed * 0.1;

    for (let i = 0; i < flakeCount; i++) {
      positions[i * 3] = (Math.random() - 0.5) * 2;
      positions[i * 3 + 1] = (Math.random() - 0.5) * 2;
      positions[i * 3 + 2] = 0;

      sizes[i] = minFlakeSize + Math.random() * (flakeSize * 100 - minFlakeSize);

      velocities[i * 2] = windX * (0.5 + Math.random() * 0.5);
      velocities[i * 2 + 1] = windY * (0.5 + Math.random() * 0.5);
    }

    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1));

    // Create shader material
    const vertexShader = `
      attribute float size;
      varying float vSize;

      void main() {
        vSize = size;
        vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
        gl_PointSize = size;
        gl_Position = projectionMatrix * mvPosition;
      }
    `;

    const fragmentShader = `
      uniform vec3 color;
      uniform float brightness;
      uniform float pixelSize;
      uniform int variant;
      varying float vSize;

      void main() {
        vec2 coord = gl_PointCoord - vec2(0.5);
        float dist = length(coord);

        float alpha = 0.0;

        if (variant == 0) {
          // Square
          alpha = 1.0;
        } else if (variant == 1) {
          // Round
          alpha = 1.0 - smoothstep(0.4, 0.5, dist);
        } else if (variant == 2) {
          // Diamond
          float diamond = abs(coord.x) + abs(coord.y);
          alpha = 1.0 - smoothstep(0.4, 0.5, diamond);
        }

        if (alpha < 0.01) discard;

        gl_FragColor = vec4(color * brightness, alpha);
      }
    `;

    const variantMap = { square: 0, round: 1, diamond: 2 };
    const colorObj = new THREE.Color(color);

    const material = new THREE.ShaderMaterial({
      uniforms: {
        color: { value: colorObj },
        brightness: { value: brightness },
        pixelSize: { value: pixelResolution },
        variant: { value: variantMap[variant] },
      },
      vertexShader,
      fragmentShader,
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    });

    const flakes = new THREE.Points(geometry, material);
    scene.add(flakes);
    flakesRef.current = flakes;

    // Animation loop
    const animate = () => {
      const positions = geometry.attributes.position.array as Float32Array;

      for (let i = 0; i < flakeCount; i++) {
        positions[i * 3] += velocities[i * 2] * 0.01;
        positions[i * 3 + 1] += velocities[i * 2 + 1] * 0.01;

        // Wrap around
        if (positions[i * 3] > 1) positions[i * 3] = -1;
        if (positions[i * 3] < -1) positions[i * 3] = 1;
        if (positions[i * 3 + 1] > 1) positions[i * 3 + 1] = -1;
        if (positions[i * 3 + 1] < -1) positions[i * 3 + 1] = 1;
      }

      geometry.attributes.position.needsUpdate = true;
      renderer.render(scene, camera);
      animationFrameRef.current = requestAnimationFrame(animate);
    };

    animate();

    // Handle resize
    const handleResize = () => {
      if (!containerRef.current || !renderer || !camera) return;
      const width = containerRef.current.clientWidth;
      const height = containerRef.current.clientHeight;
      renderer.setSize(width, height);
    };

    window.addEventListener('resize', handleResize);

    // Cleanup
    return () => {
      window.removeEventListener('resize', handleResize);
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
      }
      if (rendererRef.current && containerRef.current) {
        containerRef.current.removeChild(rendererRef.current.domElement);
      }
      geometry.dispose();
      material.dispose();
      rendererRef.current?.dispose();
    };
  }, [color, flakeSize, minFlakeSize, pixelResolution, speed, density, direction, brightness, variant]);

  return (
    <div
      ref={containerRef}
      className={`pixel-snow ${className}`}
      style={style}
    />
  );
};

export default PixelSnow;
