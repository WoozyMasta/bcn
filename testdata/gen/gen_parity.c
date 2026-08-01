/* SPDX-License-Identifier: MIT */
/* Generates fixtures with bcdec v0.98, revision
 * 93628fe5627102fe5187b7eeb99122dec6612c36. */

#define BCDEC_IMPLEMENTATION
#include "bcdec.h"

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum { ldr_count = 24, bc7_count = 32, bc6h_count = 38 };
static uint32_t state = 0x4d595df4u;

static uint32_t next_u32(void) {
  state = state * 1664525u + 1013904223u;
  return state;
}

static void fill(uint8_t *b, size_t n, unsigned i) {
  size_t j;
  if (i == 0) {
    memset(b, 0, n);
    return;
  }

  if (i == 1) {
    memset(b, 0xff, n);
    return;
  }

  if (i == 2) {
    for (j = 0; j < n; j++)
      b[j] = (uint8_t)j;
    return;
  }

  if (i == 3) {
    for (j = 0; j < n; j++)
      b[j] = (uint8_t)(0xff - j);
    return;
  }

  if (i == 4) {
    for (j = 0; j < n; j++)
      b[j] = (j & 1) ? 0x55 : 0xaa;
    return;
  }

  for (j = 0; j < n; j++)
    b[j] = (uint8_t)(next_u32() >> 24);
}

static FILE *open_out(const char *name) {
  char path[128];
  FILE *f;
  snprintf(path, sizeof(path), "testdata/parity/%s", name);

  f = fopen(path, "wb");
  if (!f) {
    perror(path);
    exit(EXIT_FAILURE);
  }

  return f;
}

static void write_out(FILE *f, const void *p, size_t n) {
  if (fwrite(p, 1, n, f) != n) {
    perror("write fixture");
    exit(EXIT_FAILURE);
  }
}

static void close_out(FILE *f) {
  if (fclose(f) != 0) {
    perror("close fixture");
    exit(EXIT_FAILURE);
  }
}

typedef void (*decode_rgba_fn)(const void *, void *, int);

static void rgba_set(const char *name, size_t size, unsigned count,
                     decode_rgba_fn decode) {
  char blocks[32], pixels[32];
  FILE *fb, *fp;
  uint8_t b[16], out[64];
  unsigned i;

  snprintf(blocks, sizeof(blocks), "%s.blocks", name);
  snprintf(pixels, sizeof(pixels), "%s.rgba", name);
  fb = open_out(blocks);
  fp = open_out(pixels);

  for (i = 0; i < count; i++) {
    fill(b, size, i);
    decode(b, out, 16);
    write_out(fb, b, size);
    write_out(fp, out, sizeof(out));
  }

  close_out(fb);
  close_out(fp);
}

static void bc4_set(void) {
  FILE *fb = open_out("bc4.blocks"), *fp = open_out("bc4.r");
  uint8_t b[8], out[16];
  unsigned i;

  for (i = 0; i < ldr_count; i++) {
    fill(b, 8, i);
    bcdec_bc4(b, out, 4);
    write_out(fb, b, 8);
    write_out(fp, out, 16);
  }

  close_out(fb);
  close_out(fp);
}

static void bc5_set(void) {
  FILE *fb = open_out("bc5.blocks"), *fp = open_out("bc5.rg");
  uint8_t b[16], out[32];
  unsigned i;

  for (i = 0; i < ldr_count; i++) {
    fill(b, 16, i);
    bcdec_bc5(b, out, 8);
    write_out(fb, b, 16);
    write_out(fp, out, 32);
  }

  close_out(fb);
  close_out(fp);
}

static void bc7_set(void) {
  FILE *fb = open_out("bc7.blocks"), *fp = open_out("bc7.rgba");
  uint8_t b[16], out[64];
  unsigned i;

  for (i = 0; i < bc7_count; i++) {
    fill(b, 16, i);
    if (i < 8)
      b[0] = (uint8_t)(1u << i);
    bcdec_bc7(b, out, 16);
    write_out(fb, b, 16);
    write_out(fp, out, 64);
  }

  close_out(fb);
  close_out(fp);
}

static void put_u16le(FILE *f, uint16_t v) {
  uint8_t b[2] = {(uint8_t)v, (uint8_t)(v >> 8)};
  write_out(f, b, 2);
}

static void bc6h_set(void) {
  static const uint8_t modes[14] = {
      0, 1, 2, 6, 10, 14, 18, 22, 26, 30, 3, 7, 11, 15,
  };

  FILE *fb = open_out("bc6h.blocks"), *fu = open_out("bc6hu.rgb16le"),
       *fs = open_out("bc6hs.rgb16le");

  uint8_t b[16];
  uint16_t out[4][12];
  unsigned i, j;

  for (i = 0; i < bc6h_count; i++) {
    fill(b, 16, i);
    if (i < 14)
      b[0] = modes[i];
    write_out(fb, b, 16);
    bcdec_bc6h_half(b, out, 12, 0);

    for (j = 0; j < 48; j++)
      put_u16le(fu, ((uint16_t *)out)[j]);
    bcdec_bc6h_half(b, out, 12, 1);

    for (j = 0; j < 48; j++)
      put_u16le(fs, ((uint16_t *)out)[j]);
  }

  close_out(fb);
  close_out(fu);
  close_out(fs);
}

int main(void) {
  rgba_set("bc1", 8, ldr_count, bcdec_bc1);
  rgba_set("bc2", 16, ldr_count, bcdec_bc2);
  rgba_set("bc3", 16, ldr_count, bcdec_bc3);
  bc4_set();
  bc5_set();
  bc6h_set();
  bc7_set();

  return EXIT_SUCCESS;
}
