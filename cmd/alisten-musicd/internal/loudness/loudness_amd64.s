//go:build amd64

#include "textflag.h"

// st (*[2][2]biquadState) 内存布局: biquadState 占 72 字节
//   {b0=0 b1=8 b2=16 a1=24 a2=32 x1=40 x2=48 y1=56 y2=64},
// L 搁架=0, L 高通=72, R 搁架=144, R 高通=216。
// L/R 同名字段相距 144 字节, 用 VMOVSD+VMOVHPD 组合成 [L,R] 向量。
//
// 寄存器分配 (128-bit VEX, lane0=L lane1=R):
//   X0  当前帧输入 x / 收尾暂存
//   X1  一级输出 y / 暂存
//   X2  二级输出 z / 暂存
//   X3  暂存
//   X4  暂存
//   X5  绝对值掩码 0x7fffffffffffffff
//   X6  峰值累加器
//   X7  能量累加器 (段内)
//   X8-X11   一级状态 x1 x2 y1 y2
//   X12-X15  二级状态 u1 u2 v1 v2

// func stereoProcessAVX2(samples []float64, st *[2][2]biquadState, sums *[2]float64, peak *float64)
TEXT ·stereoProcessAVX2(SB), NOSPLIT, $0-48
	MOVQ samples_base+0(FP), SI
	MOVQ samples_len+8(FP), CX
	MOVQ st+24(FP), DI
	MOVQ sums+32(FP), R8
	MOVQ peak+40(FP), R9

	// X5 = |x| 掩码 (VPBROADCASTQ 为 AVX2 指令)
	MOVQ $0x7fffffffffffffff, AX
	MOVQ AX, X5
	VPBROADCASTQ X5, X5

	// X6 = peak 广播到两个 lane
	MOVSD       (R9), X6
	VPBROADCASTQ X6, X6

	VXORPD X7, X7, X7 // 能量累加器清零

	// 载入滤波器状态: VMOVSD 取 L, VMOVHPD 合并 R
	VMOVSD  40(DI), X8
	VMOVHPD 184(DI), X8, X8 // x1
	VMOVSD  48(DI), X9
	VMOVHPD 192(DI), X9, X9 // x2
	VMOVSD  56(DI), X10
	VMOVHPD 200(DI), X10, X10 // y1
	VMOVSD  64(DI), X11
	VMOVHPD 208(DI), X11, X11 // y2
	VMOVSD  112(DI), X12
	VMOVHPD 256(DI), X12, X12 // u1
	VMOVSD  120(DI), X13
	VMOVHPD 264(DI), X13, X13 // u2
	VMOVSD  128(DI), X14
	VMOVHPD 272(DI), X14, X14 // v1
	VMOVSD  136(DI), X15
	VMOVHPD 280(DI), X15, X15 // v2

	TESTQ CX, CX
	JEQ   done

loop:
	VMOVUPD (SI), X0 // x = [L, R]

	// peak = max(peak, |x|)
	VANDPD X5, X0, X1
	VMAXPD X1, X6, X6

	// 一级 (高频搁架): y = b0*x + b1*x1 + b2*x2 - a1*y1 - a2*y2
	VMOVSD  0(DI), X1
	VMOVHPD 144(DI), X1, X1
	VMULPD  X0, X1, X1
	VMOVSD  8(DI), X2
	VMOVHPD 152(DI), X2, X2
	VMULPD  X8, X2, X2
	VADDPD  X2, X1, X1
	VMOVSD  16(DI), X3
	VMOVHPD 160(DI), X3, X3
	VMULPD  X9, X3, X3
	VADDPD  X3, X1, X1
	VMOVSD  24(DI), X4
	VMOVHPD 168(DI), X4, X4
	VMULPD  X10, X4, X4
	VSUBPD  X4, X1, X1
	VMOVSD  32(DI), X4
	VMOVHPD 176(DI), X4, X4
	VMULPD  X11, X4, X4
	VSUBPD  X4, X1, X1 // X1 = y

	// 一级状态移位: x2=x1, x1=x; y2=y1, y1=y
	VMOVAPS X8, X9
	VMOVAPS X0, X8
	VMOVAPS X10, X11
	VMOVAPS X1, X10

	// 二级 (高通): z = c0*y + c1*u1 + c2*u2 - d1*v1 - d2*v2
	VMOVSD  72(DI), X2
	VMOVHPD 216(DI), X2, X2
	VMULPD  X1, X2, X2
	VMOVSD  80(DI), X3
	VMOVHPD 224(DI), X3, X3
	VMULPD  X12, X3, X3
	VADDPD  X3, X2, X2
	VMOVSD  88(DI), X4
	VMOVHPD 232(DI), X4, X4
	VMULPD  X13, X4, X4
	VADDPD  X4, X2, X2
	VMOVSD  96(DI), X4
	VMOVHPD 240(DI), X4, X4
	VMULPD  X14, X4, X4
	VSUBPD  X4, X2, X2
	VMOVSD  104(DI), X4
	VMOVHPD 248(DI), X4, X4
	VMULPD  X15, X4, X4
	VSUBPD  X4, X2, X2 // X2 = z

	// 二级状态移位: u2=u1, u1=y; v2=v1, v1=z
	VMOVAPS X12, X13
	VMOVAPS X1, X12
	VMOVAPS X14, X15
	VMOVAPS X2, X14

	// 能量: sums += z*z
	VMULPD X2, X2, X3
	VADDPD X3, X7, X7

	ADDQ $16, SI
	SUBQ $2, CX
	JNE  loop

done:
	// 写回滤波器状态: VMOVSD 存 L, VMOVHPD 存 R
	VMOVSD  X8, 40(DI)
	VMOVHPD X8, 184(DI)
	VMOVSD  X9, 48(DI)
	VMOVHPD X9, 192(DI)
	VMOVSD  X10, 56(DI)
	VMOVHPD X10, 200(DI)
	VMOVSD  X11, 64(DI)
	VMOVHPD X11, 208(DI)
	VMOVSD  X12, 112(DI)
	VMOVHPD X12, 256(DI)
	VMOVSD  X13, 120(DI)
	VMOVHPD X13, 264(DI)
	VMOVSD  X14, 128(DI)
	VMOVHPD X14, 272(DI)
	VMOVSD  X15, 136(DI)
	VMOVHPD X15, 280(DI)

	// 峰值: 两个 lane 取标量最大后写回 (初值已含旧 peak)
	VUNPCKHPD X6, X6, X0
	VMAXSD    X0, X6, X6
	MOVSD     X6, (R9)

	// 段能量: *sums += X7
	VMOVUPD (R8), X0
	VADDPD X0, X7, X7
	VMOVUPD X7, (R8)

	VZEROUPPER
	RET
