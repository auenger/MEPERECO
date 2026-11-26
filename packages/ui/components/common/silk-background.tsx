"use client";

import { useEffect, useRef } from "react";
import "./silk-background.css";

const vertexSource = `
  attribute vec2 position;
  varying vec2 uv;
  void main() {
    uv = position * 0.5 + 0.5;
    gl_Position = vec4(position, 0., 1.);
  }
`;

const fragmentSource = `
  precision mediump float;
  varying vec2 uv;
  uniform float time;
  uniform float aspect;
  uniform float pearl;
  uniform vec2 pointer;

  void main() {
    vec2 p = (uv - 0.5) * vec2(aspect, 1.0) * 2.0;
    vec2 hand = p - pointer * vec2(aspect, 1.0);
    float tug = exp(-1.8 * dot(hand, hand));
    float sway = sin(time + 1.35*p.x - 0.5*p.y);
    float gather = sin(1.75*p.x + 0.7*p.y - time*0.72);
    float fold = 5.8*(0.46*p.x + 0.88*p.y + 0.34*sway + 0.23*gather)
               + 0.95*sin(p.x*2.8 - p.y + time*0.55) + 0.68*tug;
    fold += 0.40*sin(fold + p.x - time*0.45);
    float roll = 0.5 + 0.5*sin(fold);
    float crest = 0.5 + 0.5*cos(fold - 0.25);
    float sheen = pow(crest, 7.0) * (0.70 + 0.30*sin(1.5*p.x + time + 0.6*tug));
    float seam = pow(0.5 + 0.5*cos(fold + 0.48), 24.0);
    vec3 shadow = mix(vec3(0.025,0.053,0.105), vec3(0.47,0.61,0.76), pearl);
    vec3 body = mix(vec3(0.16,0.29,0.46), vec3(0.83,0.89,0.95), pearl);
    vec3 highlight = mix(vec3(0.61,0.78,0.94), vec3(0.98,0.99,1.0), pearl);
    vec3 silk = mix(shadow, body, roll);
    silk = mix(silk, highlight, 0.75*sheen + 0.20*seam);
    gl_FragColor = vec4(silk, 1.0);
  }
`;

function compile(gl: WebGLRenderingContext, type: number, source: string) {
  const shader = gl.createShader(type);
  if (!shader) return null;
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (gl.getShaderParameter(shader, gl.COMPILE_STATUS)) return shader;
  gl.deleteShader(shader);
  return null;
}

export function SilkBackground({ className }: { className: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const surface = canvas.parentElement!;
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const gl = canvas.getContext("webgl", {
      alpha: false, antialias: false, depth: false, stencil: false, powerPreference: "low-power",
    });
    if (!gl) return;

    let frame = 0;
    let previous = 0;
    let elapsed = 0;
    let visible = false;
    let pointerX = 0;
    let pointerY = 0;
    let targetX = 0;
    let targetY = 0;
    let palette = 0;
    let program: WebGLProgram | null = null;
    let buffer: WebGLBuffer | null = null;
    let time: WebGLUniformLocation | null = null;
    let aspect: WebGLUniformLocation | null = null;
    let pearl: WebGLUniformLocation | null = null;
    let pointer: WebGLUniformLocation | null = null;

    function paint() {
      if (!gl || !canvas || !program) return;
      gl.uniform1f(time, elapsed * 0.44);
      gl.uniform1f(aspect, canvas.width / canvas.height);
      gl.uniform1f(pearl, palette);
      gl.uniform2f(pointer, pointerX, pointerY);
      gl.drawArrays(gl.TRIANGLES, 0, 3);
    }

    function tick(now: number) {
      frame = requestAnimationFrame(tick);
      const delta = now - previous;
      if (delta < 1000 / 30 - 1) return;
      elapsed += Math.min(delta, 100) / 1000;
      previous = now;
      pointerX += (targetX - pointerX) * 0.09;
      pointerY += (targetY - pointerY) * 0.09;
      paint();
    }

    function sync() {
      cancelAnimationFrame(frame);
      if (program && visible && !document.hidden && !motion.matches) {
        previous = performance.now();
        frame = requestAnimationFrame(tick);
      }
    }

    function resize() {
      if (!canvas || !gl) return;
      const { width, height } = canvas.getBoundingClientRect();
      if (!width || !height) return;
      const scale = Math.min(1, Math.sqrt(230400 / (width * height)));
      canvas.width = Math.max(1, Math.round(width * scale));
      canvas.height = Math.max(1, Math.round(height * scale));
      gl.viewport(0, 0, canvas.width, canvas.height);
      paint();
    }

    function updatePalette() {
      palette = Number(getComputedStyle(surface).getPropertyValue("--silk-pearl").trim());
      paint();
    }

    function initialize() {
      if (!gl) return;
      const vertex = compile(gl, gl.VERTEX_SHADER, vertexSource);
      const fragment = compile(gl, gl.FRAGMENT_SHADER, fragmentSource);
      const linked = gl.createProgram();
      if (linked && vertex && fragment) {
        gl.attachShader(linked, vertex);
        gl.attachShader(linked, fragment);
        gl.linkProgram(linked);
      }
      gl.deleteShader(vertex);
      gl.deleteShader(fragment);
      if (!linked || !gl.getProgramParameter(linked, gl.LINK_STATUS)) {
        gl.deleteProgram(linked);
        return;
      }
      program = linked;
      gl.useProgram(program);
      buffer = gl.createBuffer();
      gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
      gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1, 3,-1, -1,3]), gl.STATIC_DRAW);
      const position = gl.getAttribLocation(program, "position");
      gl.enableVertexAttribArray(position);
      gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);
      time = gl.getUniformLocation(program, "time");
      aspect = gl.getUniformLocation(program, "aspect");
      pearl = gl.getUniformLocation(program, "pearl");
      pointer = gl.getUniformLocation(program, "pointer");
      updatePalette();
      resize();
      surface.dataset.live = "true";
      sync();
    }

    function loseContext(event: Event) {
      event.preventDefault();
      cancelAnimationFrame(frame);
      program = null;
      buffer = null;
      delete surface.dataset.live;
    }
    function move(event: PointerEvent) {
      if (!visible || motion.matches) return;
      const rect = surface.getBoundingClientRect();
      targetX = 2 * (event.clientX - rect.left) / rect.width - 1;
      targetY = 1 - 2 * (event.clientY - rect.top) / rect.height;
    }
    initialize();
    const intersection = new IntersectionObserver(([entry]) => {
      visible = !!entry?.isIntersecting;
      sync();
    });
    intersection.observe(surface);
    const size = new ResizeObserver(resize);
    size.observe(surface);
    const theme = new MutationObserver(updatePalette);
    theme.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    const landing = surface.closest("[data-theme]");
    if (landing) theme.observe(landing, { attributes: true, attributeFilter: ["data-theme"] });
    window.addEventListener("pointermove", move, { passive: true });
    document.addEventListener("visibilitychange", sync);
    motion.addEventListener("change", sync);
    canvas.addEventListener("webglcontextlost", loseContext);
    canvas.addEventListener("webglcontextrestored", initialize);
    return () => {
      cancelAnimationFrame(frame);
      intersection.disconnect();
      size.disconnect();
      theme.disconnect();
      window.removeEventListener("pointermove", move);
      document.removeEventListener("visibilitychange", sync);
      motion.removeEventListener("change", sync);
      canvas.removeEventListener("webglcontextlost", loseContext);
      canvas.removeEventListener("webglcontextrestored", initialize);
      delete surface.dataset.live;
      gl.deleteBuffer(buffer);
      gl.deleteProgram(program);
    };
  }, []);

  return (
    <div className={`silk-background ${className}`} aria-hidden="true">
      <canvas ref={canvasRef} />
    </div>
  );
}
