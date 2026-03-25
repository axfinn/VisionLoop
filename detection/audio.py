"""
音频检测器
基于频谱分析检测哭声和异响

注意: 当前版本从视频帧中提取音频信息
实际项目中音频应从独立音频流获取
"""

import os
import sys
import numpy as np

try:
    import scipy.signal as signal
    HAS_SCIPY = True
except ImportError:
    HAS_SCIPY = False
    print("[Audio] Warning: scipy not installed")


class AudioDetector:
    """音频检测器"""

    def __init__(self, sample_rate=16000):
        self.sample_rate = sample_rate

        # 哭声特征频率范围 (1-4 kHz)
        self.cry_freq_low = 1000
        self.cry_freq_high = 4000

        # 异响检测阈值
        self.noise_threshold = 0.3

        # 历史帧缓存
        self.history = []
        self.history_size = 10

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

        # 计算图像统计特征作为伪音频特征
        # 这是一个非常简化的实现
        features = self._extract_features(img)

        # 检测哭声
        cry_result = self._detect_cry(features)
        if cry_result:
            results.append(cry_result)

        # 检测异响
        noise_result = self._detect_noise(features)
        if noise_result:
            results.append(noise_result)

        # 更新历史
        self.history.append(features)
        if len(self.history) > self.history_size:
            self.history.pop(0)

        return results

    def _extract_features(self, img):
        """提取特征"""
        # 简化的频谱特征
        # 实际应该使用STFT
        gray = np.mean(img, axis=2)

        # 计算水平方向的能量
        energy = np.sum(gray ** 2, axis=1)

        # 简化的频率分析 (实际上是空间频率)
        if HAS_SCIPY:
            # 使用差分近似导数
            diff = np.diff(gray, axis=0)
            diff_energy = np.sum(diff ** 2, axis=(0, 1))
        else:
            diff_energy = float(np.var(gray))

        return {
            "energy": float(np.mean(energy)),
            "variance": float(np.var(energy)),
            "diff_energy": diff_energy,
            "max_energy": float(np.max(energy)),
            "min_energy": float(np.min(energy)),
        }

    def _detect_cry(self, features):
        """
        检测哭声

        哭声特征:
        - 频率集中在1-4kHz
        - 能量波动大
        - 有规律性
        """
        if not HAS_SCIPY:
            return None

        # 简化的哭声检测
        # 实际需要分析频谱
        energy = features["energy"]
        variance = features["variance"]

        # 高能量 + 高方差 = 可能的哭声
        cry_score = (energy / 255.0) * (variance / (energy + 1e-6))

        if cry_score > 0.5:
            return {
                "type": "cry",
                "confidence": min(cry_score, 1.0),
                "frequency_range": [self.cry_freq_low, self.cry_freq_high],
            }

        return None

    def _detect_noise(self, features):
        """
        检测异响

        异响特征:
        - 突然的能量变化
        - 频谱突然变化
        """
        if len(self.history) < 2:
            return None

        # 计算与历史帧的差异
        prev_features = self.history[-2]
        current_features = features

        energy_change = abs(
            current_features["energy"] - prev_features["energy"]
        ) / (prev_features["energy"] + 1e-6)

        diff_change = abs(
            current_features["diff_energy"] - prev_features["diff_energy"]
        ) / (prev_features["diff_energy"] + 1e-6)

        # 突然变化检测
        if energy_change > self.noise_threshold or diff_change > self.noise_threshold:
            confidence = min((energy_change + diff_change) / 2, 1.0)
            if confidence > 0.3:
                return {
                    "type": "noise",
                    "confidence": confidence,
                    "energy_change": energy_change,
                    "diff_change": diff_change,
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


def main():
    """测试"""
    detector = AudioDetector()

    # 生成测试数据
    test_data = np.random.randint(0, 255, (480, 640, 3), dtype=np.uint8)

    results = detector.detect(test_data.tobytes(), 640, 480, 3)
    print(f"Audio detection results: {results}")


if __name__ == "__main__":
    main()
