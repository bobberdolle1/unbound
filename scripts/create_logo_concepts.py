import os

# Concept A: Monolithic Frame Break
concept_a_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none">
  <!-- Solid Monolith Frame with Top-Right Sever -->
  <path d="M 96 96 H 320 V 160 H 160 V 352 H 352 V 256 H 416 V 416 H 96 V 96 Z" fill="currentColor"/>
  <!-- Vector signal breaking out of top-right -->
  <rect x="368" y="96" width="48" height="112" fill="currentColor"/>
</svg>'''

# Concept B: Route Break (Line piercing the wall)
concept_b_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none">
  <!-- Outer Box Frame with cutouts -->
  <path d="M 96 96 H 416 V 208 H 352 V 160 H 160 V 352 H 352 V 304 H 416 V 416 H 96 V 96 Z" fill="currentColor"/>
  <!-- Traversing Beam cutting through top right -->
  <path d="M 240 208 H 480 V 272 H 240 Z" fill="currentColor"/>
</svg>'''

# Concept C: UNBOUND Monogram (Architectural U-Break)
concept_c_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none">
  <!-- Base Monolithic U-Stem -->
  <path d="M 96 96 H 168 V 344 H 344 V 240 H 416 V 344 C 416 384 384 416 344 416 H 168 C 128 416 96 384 96 344 V 96 Z" fill="currentColor"/>
  <!-- Floating Severed Top-Right Block -->
  <rect x="344" y="96" width="72" height="96" rx="8" fill="currentColor"/>
</svg>'''

print("Created concept definitions.")
