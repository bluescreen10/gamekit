//go:build windows

#include "textflag.h"

// floatThunk is called with the native calling convention of Windows on arm64:
// R0 holds a *floatFrame (see win32.go), whose layout is
//
//	0    function
//	8    ints[8], the arguments in R0-R7
//	72   floats[8], the arguments in F0-F7
//	136  result, from R0
//	144  floatResult, from F0
//
// It saves the frame pointer, the link register, and R19, which holds the
// frame during the call and which the callee preserves.
TEXT ·floatThunk(SB), NOSPLIT|NOFRAME, $0-0
	STP.W	(R29, R30), -32(RSP)
	MOVD	RSP, R29
	MOVD	R19, 16(RSP)
	MOVD	R0, R19
	FLDPD	72(R19), (F0, F1)
	FLDPD	88(R19), (F2, F3)
	FLDPD	104(R19), (F4, F5)
	FLDPD	120(R19), (F6, F7)
	MOVD	(R19), R16
	LDP	8(R19), (R0, R1)
	LDP	24(R19), (R2, R3)
	LDP	40(R19), (R4, R5)
	LDP	56(R19), (R6, R7)
	CALL	(R16)
	MOVD	R0, 136(R19)
	FMOVD	F0, 144(R19)
	MOVD	16(RSP), R19
	LDP.P	32(RSP), (R29, R30)
	RET

// func floatThunkAddress() uintptr
TEXT ·floatThunkAddress(SB), NOSPLIT, $0-8
	MOVD	$·floatThunk(SB), R0
	MOVD	R0, ret+0(FP)
	RET
