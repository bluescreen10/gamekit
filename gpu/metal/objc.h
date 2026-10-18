#ifndef GAMEKIT_METAL_OBJC_H
#define GAMEKIT_METAL_OBJC_H

// Typed objc_msgSend shims. Objective-C has no functions to call, only messages, and
// objc_msgSend must be called through a cast to the receiving method's exact type. Each
// shim is that cast for one method signature, and nothing else: the Go side picks the
// selector, and every object and selector crosses as a uintptr_t, which cgo cannot
// mistake for Go memory and need not size (it cannot size id in Objective-C mode).

#import <Metal/Metal.h>
#import <QuartzCore/QuartzCore.h>
#include <objc/message.h>
#include <mach/mach_time.h>
#include <objc/runtime.h>
#include <stdint.h>

typedef uintptr_t gk_obj;
typedef uintptr_t gk_sel;
typedef NSUInteger gk_u;

#define GK_SEND(ret, ...) ((ret (*)(id, SEL, ##__VA_ARGS__))objc_msgSend)

// Functions whose Objective-C object types cgo cannot size.
static inline gk_obj gk_device(void) {
    return (gk_obj)MTLCreateSystemDefaultDevice();
}
static inline gk_obj gk_dispatch_data(const void *p, size_t n) {
    return (gk_obj)dispatch_data_create(p, n, NULL, DISPATCH_DATA_DESTRUCTOR_DEFAULT);
}

// Messages returning an object.
static inline gk_obj gk_o(gk_obj o, gk_sel s) {
    return (gk_obj)GK_SEND(id)((id)o, (SEL)s);
}
static inline gk_obj gk_o_o(gk_obj o, gk_sel s, gk_obj a) {
    return (gk_obj)GK_SEND(id, id)((id)o, (SEL)s, (id)a);
}
static inline gk_obj gk_o_u(gk_obj o, gk_sel s, gk_u a) {
    return (gk_obj)GK_SEND(id, gk_u)((id)o, (SEL)s, a);
}
static inline gk_obj gk_o_uu(gk_obj o, gk_sel s, gk_u a, gk_u b) {
    return (gk_obj)GK_SEND(id, gk_u, gk_u)((id)o, (SEL)s, a, b);
}
static inline gk_obj gk_o_op(gk_obj o, gk_sel s, gk_obj a, gk_obj *error) {
    return (gk_obj)GK_SEND(id, id, id *)((id)o, (SEL)s, (id)a, (id *)error);
}
static inline gk_obj gk_o_oop(gk_obj o, gk_sel s, gk_obj a, gk_obj b, gk_obj *error) {
    return (gk_obj)GK_SEND(id, id, id, id *)((id)o, (SEL)s, (id)a, (id)b, (id *)error);
}
static inline gk_obj gk_o_uurr(gk_obj o, gk_sel s, gk_u a, gk_u b, NSRange c, NSRange d) {
    return (gk_obj)GK_SEND(id, gk_u, gk_u, NSRange, NSRange)((id)o, (SEL)s, a, b, c, d);
}
static inline gk_obj gk_o_puu(gk_obj o, gk_sel s, const void *a, gk_u b, gk_u c) {
    return (gk_obj)GK_SEND(id, const void *, gk_u, gk_u)((id)o, (SEL)s, a, b, c);
}
static inline gk_obj gk_o_r(gk_obj o, gk_sel s, NSRange a) {
    return (gk_obj)GK_SEND(id, NSRange)((id)o, (SEL)s, a);
}

// Messages returning a scalar, a pointer, or a struct.
static inline gk_u gk_u_(gk_obj o, gk_sel s) {
    return GK_SEND(gk_u)((id)o, (SEL)s);
}
// BOOL is bool on arm64 but signed char on x86_64, so these return an int.
static inline int gk_b(gk_obj o, gk_sel s) {
    return GK_SEND(BOOL)((id)o, (SEL)s);
}
static inline int gk_b_u(gk_obj o, gk_sel s, gk_u a) {
    return GK_SEND(BOOL, gk_u)((id)o, (SEL)s, a);
}
static inline int gk_b_o(gk_obj o, gk_sel s, gk_obj a) {
    return GK_SEND(BOOL, id)((id)o, (SEL)s, (id)a);
}
static inline int gk_b_uu(gk_obj o, gk_sel s, gk_u a, gk_u b) {
    return GK_SEND(BOOL, gk_u, gk_u)((id)o, (SEL)s, a, b);
}
static inline void *gk_p(gk_obj o, gk_sel s) {
    return GK_SEND(void *)((id)o, (SEL)s);
}
static inline const char *gk_str(gk_obj o, gk_sel s) {
    return GK_SEND(const char *)((id)o, (SEL)s);
}
static inline double gk_d(gk_obj o, gk_sel s) {
    return GK_SEND(double)((id)o, (SEL)s);
}
static inline uint64_t gk_rid(gk_obj o, gk_sel s) {
    return GK_SEND(MTLResourceID)((id)o, (SEL)s)._impl;
}
static inline CGSize gk_size(gk_obj o, gk_sel s) {
    return GK_SEND(CGSize)((id)o, (SEL)s);
}
// A struct too large for registers comes back through memory the caller supplies,
// which on x86_64 takes objc_msgSend_stret. CGRect is one; arm64 has no such variant.
static inline CGRect gk_rect(gk_obj o, gk_sel s) {
#if defined(__x86_64__)
    return ((CGRect (*)(id, SEL))objc_msgSend_stret)((id)o, (SEL)s);
#else
    return GK_SEND(CGRect)((id)o, (SEL)s);
#endif
}

// Messages returning nothing.
static inline void gk_v(gk_obj o, gk_sel s) {
    GK_SEND(void)((id)o, (SEL)s);
}
static inline void gk_v_o(gk_obj o, gk_sel s, gk_obj a) {
    GK_SEND(void, id)((id)o, (SEL)s, (id)a);
}
static inline void gk_v_u(gk_obj o, gk_sel s, gk_u a) {
    GK_SEND(void, gk_u)((id)o, (SEL)s, a);
}
static inline void gk_v_d(gk_obj o, gk_sel s, double a) {
    GK_SEND(void, double)((id)o, (SEL)s, a);
}
static inline void gk_v_ou(gk_obj o, gk_sel s, gk_obj a, gk_u b) {
    GK_SEND(void, id, gk_u)((id)o, (SEL)s, (id)a, b);
}
static inline void gk_v_uu(gk_obj o, gk_sel s, gk_u a, gk_u b) {
    GK_SEND(void, gk_u, gk_u)((id)o, (SEL)s, a, b);
}
static inline void gk_v_fff(gk_obj o, gk_sel s, float a, float b, float c) {
    GK_SEND(void, float, float, float)((id)o, (SEL)s, a, b, c);
}
static inline void gk_v_pp(gk_obj o, gk_sel s, void *a, void *b) {
    GK_SEND(void, void *, void *)((id)o, (SEL)s, a, b);
}
static inline void gk_v_puo(gk_obj o, gk_sel s, const gk_obj *a, gk_u b, gk_obj c) {
    GK_SEND(void, const id *, gk_u, id)((id)o, (SEL)s, (const id *)a, b, (id)c);
}
static inline void gk_v_puu(gk_obj o, gk_sel s, const void *a, gk_u b, gk_u c) {
    GK_SEND(void, const void *, gk_u, gk_u)((id)o, (SEL)s, a, b, c);
}
static inline void gk_v_uuu(gk_obj o, gk_sel s, gk_u a, gk_u b, gk_u c) {
    GK_SEND(void, gk_u, gk_u, gk_u)((id)o, (SEL)s, a, b, c);
}
static inline void gk_v_uuuuu(gk_obj o, gk_sel s, gk_u a, gk_u b, gk_u c, gk_u d, gk_u e) {
    GK_SEND(void, gk_u, gk_u, gk_u, gk_u, gk_u)((id)o, (SEL)s, a, b, c, d, e);
}
static inline void gk_v_uuuuuuuu(gk_obj o, gk_sel s, gk_u a, gk_u b, gk_u c, gk_u d, gk_u e, gk_u f, NSInteger g,
                                 gk_u h) {
    GK_SEND(void, gk_u, gk_u, gk_u, gk_u, gk_u, gk_u, NSInteger, gk_u)((id)o, (SEL)s, a, b, c, d, e, f, g, h);
}
static inline void gk_v_uou(gk_obj o, gk_sel s, gk_u a, gk_obj b, gk_u c) {
    GK_SEND(void, gk_u, id, gk_u)((id)o, (SEL)s, a, (id)b, c);
}
static inline void gk_v_uuou(gk_obj o, gk_sel s, gk_u a, gk_u b, gk_obj c, gk_u d) {
    GK_SEND(void, gk_u, gk_u, id, gk_u)((id)o, (SEL)s, a, b, (id)c, d);
}
static inline void gk_v_ououu(gk_obj o, gk_sel s, gk_obj a, gk_u b, gk_obj c, gk_u d, gk_u e) {
    GK_SEND(void, id, gk_u, id, gk_u, gk_u)((id)o, (SEL)s, (id)a, b, (id)c, d, e);
}
static inline void gk_v_r(gk_obj o, gk_sel s, NSRange a) {
    GK_SEND(void, NSRange)((id)o, (SEL)s, a);
}
static inline void gk_v_viewport(gk_obj o, gk_sel s, MTLViewport a) {
    GK_SEND(void, MTLViewport)((id)o, (SEL)s, a);
}
static inline void gk_v_scissor(gk_obj o, gk_sel s, MTLScissorRect a) {
    GK_SEND(void, MTLScissorRect)((id)o, (SEL)s, a);
}
static inline void gk_v_clear(gk_obj o, gk_sel s, MTLClearColor a) {
    GK_SEND(void, MTLClearColor)((id)o, (SEL)s, a);
}
static inline void gk_v_cgsize(gk_obj o, gk_sel s, CGSize a) {
    GK_SEND(void, CGSize)((id)o, (SEL)s, a);
}
static inline void gk_v_ss(gk_obj o, gk_sel s, MTLSize a, MTLSize b) {
    GK_SEND(void, MTLSize, MTLSize)((id)o, (SEL)s, a, b);
}
static inline void gk_v_us(gk_obj o, gk_sel s, uint64_t a, MTLSize b) {
    GK_SEND(void, uint64_t, MTLSize)((id)o, (SEL)s, a, b);
}
// copyFromBuffer:sourceOffset:sourceBytesPerRow:sourceBytesPerImage:sourceSize:
//   toTexture:destinationSlice:destinationLevel:destinationOrigin:
static inline void gk_v_ouuusouuz(gk_obj o, gk_sel s, gk_obj a, gk_u b, gk_u c, gk_u d, MTLSize e, gk_obj f, gk_u g,
                                  gk_u h, MTLOrigin i) {
    GK_SEND(void, id, gk_u, gk_u, gk_u, MTLSize, id, gk_u, gk_u, MTLOrigin)
    ((id)o, (SEL)s, (id)a, b, c, d, e, (id)f, g, h, i);
}
// copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:
//   toBuffer:destinationOffset:destinationBytesPerRow:destinationBytesPerImage:
static inline void gk_v_ouuzsouuu(gk_obj o, gk_sel s, gk_obj a, gk_u b, gk_u c, MTLOrigin d, MTLSize e, gk_obj f,
                                  gk_u g, gk_u h, gk_u i) {
    GK_SEND(void, id, gk_u, gk_u, MTLOrigin, MTLSize, id, gk_u, gk_u, gk_u)
    ((id)o, (SEL)s, (id)a, b, c, d, e, (id)f, g, h, i);
}
// copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:
//   toTexture:destinationSlice:destinationLevel:destinationOrigin:
static inline void gk_v_ouuzsouuz(gk_obj o, gk_sel s, gk_obj a, gk_u b, gk_u c, MTLOrigin d, MTLSize e, gk_obj f,
                                  gk_u g, gk_u h, MTLOrigin i) {
    GK_SEND(void, id, gk_u, gk_u, MTLOrigin, MTLSize, id, gk_u, gk_u, MTLOrigin)
    ((id)o, (SEL)s, (id)a, b, c, d, e, (id)f, g, h, i);
}

// gk_add_feedback_handler adds the commit feedback handler: a block, which only
// Objective-C can make, that hands each feedback object to the Go side with ctx.
extern void gkMetalFeedback(uintptr_t ctx, uintptr_t feedback);
static inline void gk_add_feedback_handler(gk_obj options, gk_sel s, uintptr_t ctx) {
    GK_SEND(void, void (^)(id))((id)options, (SEL)s, ^(id feedback) {
      gkMetalFeedback(ctx, (uintptr_t)feedback);
    });
}

// The autorelease pool entry points. They are public API of the runtime but declared
// only in its private headers.
extern void *objc_autoreleasePoolPush(void);
extern void objc_autoreleasePoolPop(void *pool);

#endif
