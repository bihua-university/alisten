#!/usr/bin/env python3
"""
图标生成脚本
从 icon.png 生成不同尺寸的图标文件，用于 PWA、Apple Touch Icon、favicon 等场景
"""

import os
import sys
from PIL import Image

def generate_icons():
    """生成各种尺寸的图标文件"""
    
    # 源图标文件路径
    source_icon = "icon.png"
    
    # 检查源文件是否存在
    if not os.path.exists(source_icon):
        print(f"错误: 源文件 {source_icon} 不存在")
        return False
    
    # 需要生成的图标尺寸 (覆盖各种使用场景)
    icon_sizes = [
        # 16,   # favicon 小尺寸 - 未使用
        # 32,   # favicon 标准尺寸 - 未使用
        # 48,   # Windows 应用图标 - 未使用
        # 64,   # 高分辨率 favicon - 未使用
        # 72,   # Android Chrome (ldpi) - 未使用
        # 96,   # Android Chrome (mdpi) - 未使用
        # 120,  # iOS Safari (iPhone) - 未使用
        # 128,  # Chrome Web Store - 未使用
        # 144,  # Android Chrome (xhdpi) - 未使用
        # 152,  # iOS Safari (iPad) - 未使用
        180,  # iOS Safari (iPhone Plus) - 项目中使用 (apple-touch-icon)
        192,  # Android Chrome (xxhdpi) / PWA 标准 - 项目中使用
        # 256,  # Windows 10 应用 - 未使用
        # 384,  # PWA 中等尺寸 - 未使用
        512,  # PWA 大尺寸 / Android Chrome (xxxhdpi) - 项目中使用
        # 1024, # iOS App Store / 高质量图标 - 未使用
    ]
    
    try:
        # 打开源图像
        print(f"正在打开源图像: {source_icon}")
        with Image.open(source_icon) as img:
            # 确保图像是 RGBA 模式以保持透明度
            if img.mode != 'RGBA':
                img = img.convert('RGBA')
            
            print(f"源图像尺寸: {img.size}")
            
            # 为每个尺寸生成图标
            for size in icon_sizes:
                output_path = f"icon-{size}x{size}.png"
                
                # 使用高质量重采样算法调整图像大小
                resized_img = img.resize((size, size), Image.Resampling.LANCZOS)
                
                # 保存图像
                resized_img.save(output_path, "PNG", optimize=True)
                print(f"已生成: {output_path}")
            
            print(f"\n✅ 成功生成 {len(icon_sizes)} 个图标文件")
            print("\n生成的文件列表:")
            for size in icon_sizes:
                print(f"  - icon-{size}x{size}.png")
            
            print("\n📱 使用场景说明:")
            print("  • 180x180: iOS Safari (apple-touch-icon) - 项目中使用")
            print("  • 192x192: PWA 标准尺寸 - 项目中使用")
            print("  • 512x512: PWA 大尺寸 - 项目中使用")
            
            return True
            
    except Exception as e:
        print(f"错误: 生成图标时发生异常: {e}")
        return False

def main():
    """主函数"""
    print("🎨 图标生成器")
    print("=" * 50)
    
    # 检查是否安装了 Pillow
    try:
        from PIL import Image
    except ImportError:
        print("错误: 未安装 Pillow 库")
        print("请运行: pip install Pillow")
        sys.exit(1)
    
    # 生成图标
    success = generate_icons()
    
    if success:
        print("\n🎉 图标生成完成!")
        print("\n💡 提示:")
        print("  • 生成的图标文件已保存在当前目录下")
        print("  • 项目已配置使用 icon-32x32.png (favicon) 和 icon-180x180.png (apple-touch-icon)")
        print("  • PWA 应用建议使用 192x192 和 512x512 尺寸")
    else:
        print("\n❌ 图标生成失败!")
        sys.exit(1)

if __name__ == "__main__":
    main()