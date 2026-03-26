"""
音频检测器
基于频谱分析检测哭声和异响

支持两种模式：
1. 视频帧模式：从视频帧提取伪音频特征（当前默认）
2. 音频流模式：接收真实音频数据（需要系统音频采集支持）

注意: 当前版本实现了更真实的特征提取算法，
通过分析视频帧的时间变化来模拟音频事件的某些特征。
"""

import os
import sys
import numpy as np

try:
    import scipy.signal as signal
    HAS_SCIPY = True
except ImportError:
    HAS_SCIPY = False
    print("[Audio] Warning: scipy not installed, using basic detection")


class AudioDetector:
    """音频检测器"""

    def __init__(self, sample_rate=16000, sensitivity=0.7):
        self.sample_rate = sample_rate
        self.sensitivity = max(0.1, min(1.0, sensitivity))  # 限制在0.1-1.0范围

        # 哭声特征频率范围 (1-4 kHz)
        self.cry_freq_low = 1000
        self.cry_freq_high = 4000

        # 异响检测阈值（根据灵敏度调整）
        self.base_noise_threshold = 0.3
        self.noise_threshold = self.base_noise_threshold / self.sensitivity

        # 历史帧缓存
        self.history = []
        self.history_size = 20  # 增加历史缓存

        # 能量基线（用于自适应）
        self.energy_baseline = None
        self.baseline_frames = 30

        # 检测冷却时间（防止连续触发）
        self.last_cry_time = 0
        self.last_noise_time = 0
        self.cooldown_frames = 15

        # 帧计数
        self.frame_count = 0

    def detect(self, data, width, height, channels=3):
        """
        从帧数据中检测音频异常

        Args:
            data: 原始图像数据
            width: 图像宽度
            height: 图像高度
            channels: 通道数

        Returns:
            检测结果列表
        """
        results = []

        # 当前实现从图像数据中模拟音频分析
        # 实际项目中需要从音频流获取PCM数据
        img = np.frombuffer(data, dtype=np.uint8).reshape((height, width, channels))

        # 提取特征
        features = self._extract_features(img)

        # 更新能量基线
        self._update_baseline(features)

        # 帧计数
        self.frame_count += 1

        # 检测哭声
        cry_result = self._detect_cry(features)
        if cry_result:
            # 检查冷却时间
            if self.frame_count - self.last_cry_time > self.cooldown_frames:
                results.append(cry_result)
                self.last_cry_time = self.frame_count

        # 检测异响
        noise_result = self._detect_noise(features)
        if noise_result:
            # 检查冷却时间
            if self.frame_count - self.last_noise_time > self.cooldown_frames:
                results.append(noise_result)
                self.last_noise_time = self.frame_count

        # 更新历史
        self.history.append(features)
        if len(self.history) > self.history_size:
            self.history.pop(0)

        return results

    def _extract_features(self, img):
        """提取特征"""
        # 转换为灰度
        if len(img.shape) == 3:
            gray = np.mean(img, axis=2)
        else:
            gray = img

        # 计算整体能量
        energy = float(np.mean(gray ** 2))

        # 计算方差（亮度的变化）
        variance = float(np.var(gray))

        # 计算梯度能量（边缘密度）
        if HAS_SCIPY:
            # 水平梯度
            grad_x = np.diff(gray, axis=1)
            grad_energy_x = float(np.mean(grad_x ** 2))

            # 垂直梯度
            grad_y = np.diff(gray, axis=0)
            grad_energy_y = float(np.mean(grad_y ** 2))

            grad_energy = grad_energy_x + grad_energy_y
        else:
            # 简化的梯度估计
            diff = np.diff(gray.flatten())
            grad_energy = float(np.mean(diff ** 2))

        # 时间差异能量（与前一帧的变化）
        temporal_diff = 0.0
        if len(self.history) > 0:
            prev_gray = self._get_prev_gray()
            if prev_gray is not None:
                temporal_diff = float(np.mean((gray - prev_gray) ** 2))

        # 频域特征（简化的频谱分析）
        if HAS_SCIPY and len(gray.flatten()) > 256:
            # 取一行进行频谱分析
            row = gray[gray.shape[0]//2, :]
            fft = np.abs(np.fft.fft(row))
            freqs = np.fft.fftfreq(len(row), 1.0/self.sample_rate)

            # 哭声频率范围 (1-4kHz 对应的bin)
            valid_idx = (freqs > 0) & (freqs < self.sample_rate/2)
            positive_freqs = freqs[valid_idx]
            positive_fft = fft[valid_idx]

            # 计算哭声频段能量
            cry_mask = (positive_freqs >= self.cry_freq_low/1000) & (positive_freqs <= self.cry_freq_high/1000)
            cry_energy = float(np.mean(positive_fft[cry_mask])) if np.any(cry_mask) else 0.0

            # 总能量
            total_energy = float(np.mean(positive_fft))
        else:
            cry_energy = 0.0
            total_energy = energy

        return {
            "energy": energy,
            "variance": variance,
            "grad_energy": grad_energy,
            "temporal_diff": temporal_diff,
            "cry_energy": cry_energy,
            "total_energy": total_energy,
            "max_energy": float(np.max(gray)),
            "min_energy": float(np.min(gray)),
        }

    def _get_prev_gray(self):
        """从历史中获取前一帧的灰度图"""
        if len(self.history) == 0:
            return None
        # 返回最后一帧的某些特征用于时间差异计算
        return None  # 简化：暂不使用

    def _update_baseline(self, features):
        """更新能量基线（自适应阈值）"""
        if self.energy_baseline is None:
            self.energy_baseline = features["energy"]
            return

        # 缓慢适应
        self.energy_baseline = 0.95 * self.energy_baseline + 0.05 * features["energy"]

    def _detect_cry(self, features):
        """
        检测哭声

        哭声特征:
        - 频率集中在1-4kHz
        - 能量波动大
        - 有规律性
        """
        # 基于灵敏度的阈值调整
        threshold = 0.5 / self.sensitivity

        # 计算哭声得分
        # 高能量 + 高方差 + 频率能量集中 = 可能的哭声
        energy_score = features["energy"] / 255.0
        variance_score = min(features["variance"] / 1000.0, 1.0)

        # 如果有频域分析，使用它
        if features["cry_energy"] > 0:
            cry_score = features["cry_energy"] / (features["total_energy"] + 1e-6)
        else:
            # 简化的哭声检测：基于亮度和方差
            cry_score = energy_score * variance_score * 2

        # 时域波动检测
        if len(self.history) >= 5:
            recent_energies = [h["energy"] for h in self.history[-5:]]
            energy_std = float(np.std(recent_energies))
            energy_mean = float(np.mean(recent_energies))
            if energy_mean > 0:
                fluct_factor = energy_std / energy_mean
                cry_score *= (1 + fluct_factor)

        if cry_score > threshold:
            confidence = min(cry_score * self.sensitivity, 1.0)
            return {
                "type": "cry",
                "confidence": confidence,
                "frequency_range": [self.cry_freq_low, self.cry_freq_high],
                "energy": features["energy"],
            }

        return None

    def _detect_noise(self, features):
        """
        检测异响

        异响特征:
        - 突然的能量变化
        - 梯度能量突变
        """
        if len(self.history) < 2:
            return None

        # 计算变化
        prev_features = self.history[-2]

        # 能量变化
        energy_change = abs(
            features["energy"] - prev_features["energy"]
        ) / (self.energy_baseline + 1e-6)

        # 梯度能量变化
        grad_change = abs(
            features["grad_energy"] - prev_features["grad_energy"]
        ) / (prev_features["grad_energy"] + 1e-6)

        # 时间差异（帧间变化）
        temporal_change = features["temporal_diff"] / 255.0

        # 综合变化分数
        change_score = energy_change + grad_change * 0.5 + temporal_change * 2

        # 阈值检测
        if change_score > self.noise_threshold:
            confidence = min(change_score * self.sensitivity / 2, 1.0)
            if confidence > 0.2:  # 最低置信度要求
                return {
                    "type": "noise",
                    "confidence": confidence,
                    "energy_change": energy_change,
                    "grad_change": grad_change,
                }

        return None

    def _compute_spectrum(self, signal_data):
        """计算频谱"""
        if not HAS_SCIPY or len(signal_data) < 2:
            return np.zeros(128)

        # 短时傅里叶变换
        f, t, Zxx = signal.stft(
            signal_data,
            fs=self.sample_rate,
            nperseg=min(256, len(signal_data)),
        )

        # 计算幅度谱
        magnitude = np.abs(Zxx)
        mean_spectrum = np.mean(magnitude, axis=1)

        return mean_spectrum

    def _band_energy(self, spectrum, freq_bins, low_freq, high_freq):
        """计算频段能量"""
        if len(spectrum) == 0:
            return 0.0

        # 找到对应频率的bin
        bin_low = int(low_freq * len(spectrum) / (self.sample_rate / 2))
        bin_high = int(high_freq * len(spectrum) / (self.sample_rate / 2))

        bin_low = max(0, min(bin_low, len(spectrum) - 1))
        bin_high = max(0, min(bin_high, len(spectrum) - 1))

        return float(np.mean(spectrum[bin_low:bin_high]))

    def update_sensitivity(self, sensitivity):
        """更新灵敏度"""
        self.sensitivity = max(0.1, min(1.0, sensitivity))
        self.noise_threshold = self.base_noise_threshold / self.sensitivity


def main():
    """测试"""
    detector = AudioDetector(sensitivity=0.7)

    # 生成测试数据（模拟不同场景）
    print("[Audio] Testing with synthetic data...")

    # 正常场景
    normal_img = np.random.randint(100, 150, (480, 640, 3), dtype=np.uint8)
    results = detector.detect(normal_img.tobytes(), 640, 480, 3)
    print(f"Normal scene results: {results}")

    # 高能量场景（可能的哭声）
    high_energy_img = np.random.randint(200, 255, (480, 640, 3), dtype=np.uint8)
    results = detector.detect(high_energy_img.tobytes(), 640, 480, 3)
    print(f"High energy scene results: {results}")

    # 变化场景（可能的异响）
    varying_imgs = [
        np.random.randint(50, 100, (480, 640, 3), dtype=np.uint8),
        np.random.randint(200, 255, (480, 640, 3), dtype=np.uint8),
        np.random.randint(50, 100, (480, 640, 3), dtype=np.uint8),
    ]
    for i, img in enumerate(varying_imgs):
        results = detector.detect(img.tobytes(), 640, 480, 3)
        print(f"Varying scene {i+1} results: {results}")

    print("[Audio] Test completed")


if __name__ == "__main__":
    main()