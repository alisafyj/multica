"use client";

import { useEffect, useRef } from "react";

const SPACING = 25;
const RADIUS = 203;
const PULL = 0.8;
const REST = 0.34;

type Dot = { hx: number; hy: number; x: number; y: number; vx: number; vy: number };

const isEditable = (node: EventTarget | null) =>
  node instanceof Element && !!node.closest('input, textarea, [contenteditable]:not([contenteditable="false"])');

/**
 * Kinetic dot grid, ported from open-design's AppWashKineticGrid: dots spring
 * back to their home cell (accel .08, damping .82) and are pulled toward the
 * cursor inside a 203px radius. Must be the first child of a `relative`
 * ancestor that defines the fill area; canvas is pointer-events:none so it
 * never intercepts input.
 */
export function DesignDotGrid() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const host = canvas?.parentElement;
    if (!canvas || !host) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dotColor = getComputedStyle(host).getPropertyValue("--muted-foreground").trim() || "#9a9a9a";
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const mouse = { x: -9999, y: -9999, active: false };
    let suppressed = false;
    let width = 1;
    let height = 1;
    let dots: Dot[] = [];

    const build = () => {
      const rect = host.getBoundingClientRect();
      width = Math.max(1, Math.floor(rect.width));
      // Fill the whole scrollable panel, not just the fold, so the grid
      // still reads once the composer content pushes the panel taller.
      height = Math.max(1, Math.floor(Math.max(rect.height, host.scrollHeight)));
      const dpr = window.devicePixelRatio || 1;
      canvas.width = Math.floor(width * dpr);
      canvas.height = Math.floor(height * dpr);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${height}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      dots = [];
      const cols = Math.floor(width / SPACING) + 2;
      const rows = Math.floor(height / SPACING) + 2;
      for (let c = 0; c < cols; c++) {
        for (let r = 0; r < rows; r++) {
          const hx = c * SPACING;
          const hy = r * SPACING;
          dots.push({ hx, hy, x: hx, y: hy, vx: 0, vy: 0 });
        }
      }
    };

    const drawStatic = () => {
      ctx.clearRect(0, 0, width, height);
      ctx.fillStyle = dotColor;
      for (const d of dots) {
        ctx.globalAlpha = REST * Math.max(0, Math.min(1, (height - d.hy) / 90));
        ctx.beginPath();
        ctx.arc(d.hx, d.hy, 0.7, 0, 2 * Math.PI);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
    };

    build();
    const ro = new ResizeObserver(() => {
      build();
      if (reduced) drawStatic();
    });
    ro.observe(host);

    if (reduced) {
      drawStatic();
      return () => ro.disconnect();
    }

    const onMove = (event: MouseEvent) => {
      const rect = canvas.getBoundingClientRect();
      mouse.x = event.clientX - rect.left;
      mouse.y = event.clientY - rect.top;
      mouse.active = true;
    };
    const onLeave = () => {
      mouse.active = false;
      mouse.x = -9999;
      mouse.y = -9999;
    };
    const onFocusIn = (event: FocusEvent) => {
      suppressed = isEditable(event.target);
    };
    const onFocusOut = () => {
      suppressed = false;
    };
    window.addEventListener("mousemove", onMove, { passive: true });
    window.addEventListener("mouseout", onLeave);
    window.addEventListener("blur", onLeave);
    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("focusout", onFocusOut);

    let raf = 0;
    const frame = () => {
      if (document.hidden) {
        raf = requestAnimationFrame(frame);
        return;
      }
      ctx.clearRect(0, 0, width, height);
      const live = mouse.active && !suppressed;
      for (const d of dots) {
        let ax = (d.hx - d.x) * 0.08;
        let ay = (d.hy - d.y) * 0.08;
        if (live) {
          const dx = mouse.x - d.x;
          const dy = mouse.y - d.y;
          const dist = Math.sqrt(dx * dx + dy * dy);
          if (dist < RADIUS && dist > 0.001) {
            const f = (1 - dist / RADIUS) * PULL;
            ax += (dx / dist) * f;
            ay += (dy / dist) * f;
          }
        }
        d.vx = (d.vx + ax) * 0.82;
        d.vy = (d.vy + ay) * 0.82;
        d.x += d.vx;
        d.y += d.vy;
        const prox = live
          ? Math.max(0, 1 - Math.sqrt((mouse.x - d.x) ** 2 + (mouse.y - d.y) ** 2) / RADIUS)
          : 0;
        const fade = Math.max(0, Math.min(1, (height - d.y) / 90));
        ctx.globalAlpha = (REST + prox * (1 - REST)) * fade;
        ctx.fillStyle = dotColor;
        ctx.beginPath();
        ctx.arc(d.x, d.y, 0.7 + prox * 0.9, 0, 2 * Math.PI);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      raf = requestAnimationFrame(frame);
    };
    raf = requestAnimationFrame(frame);

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseout", onLeave);
      window.removeEventListener("blur", onLeave);
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("focusout", onFocusOut);
    };
  }, []);

  return <canvas ref={canvasRef} aria-hidden="true" className="pointer-events-none absolute left-0 top-0 z-0" />;
}
