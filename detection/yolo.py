"""
YOLOv8 检测器
支持姿态检测和人流检测
"""

import os
import sys
import time
import numpy as np

# 尝试导入ultralytics YOLO
try:
    from ultralytics import YOLO
    HAS_ULTRALYTICS = True
except ImportError:
    HAS_ULTRALYTICS = False
    print("[YOLO] Warning: ultralytics not installed, using fallback")

try:
    import cv2
    import torch
    HAS_TORCH = True
except ImportError:
    HAS_TORCH = False
    print("[YOLO] Warning: torch/cv2 not installed")


class YOLODetector:
    """YOLOv8检测器"""

    def __init__(self, pose_model="yolov8n-pose.pt", person_model="yolov8n.pt"):
        self.pose_model_path = pose_model
        self.person_model_path = person_model

        self.pose_model = None
        self.person_model = None

        if HAS_ULTRALYTICS and HAS_TORCH:
            self._load_models()

    def _load_models(self):
        """加载模型"""
        try:
            if os.path.exists(self.pose_model_path):
                self.pose_model = YOLO(self.pose_model_path)
                print(f"[YOLO] Pose model loaded: {self.pose_model_path}")
            else:
                # 尝试从ultralytics下载
                self.pose_model = YOLO("yolov8n-pose.pt")
                print("[YOLO] Pose model downloaded")
        except Exception as e:
            print(f"[YOLO] Failed to load pose model: {e}")

        try:
            if os.path.exists(self.person_model_path):
                self.person_model = YOLO(self.person_model_path)
                print(f"[YOLO] Person model loaded: {self.person_model_path}")
            else:
                self.person_model = YOLO("yolov8n.pt")
                print("[YOLO] Person model downloaded")
        except Exception as e:
            print(f"[YOLO] Failed to load person model: {e}")

    def detect(self, data, width, height, channels=3):
        """
        检测帧中的目标

        Args:
            data: 原始图像数据 (RGB)
            width: 图像宽度
            height: 图像高度
            channels: 通道数

        Returns:
            检测结果列表
        """
        results = []

        # 转换为numpy数组
        img = np.frombuffer(data, dtype=np.uint8)
        if channels == 3:
            img = img.reshape((height, width, 3))
            # BGR to RGB
            img = img[:, :, ::-1]
        else:
            img = img.reshape((height, width))

        # 姿态检测 - 摔倒
        if self.pose_model is not None:
            pose_results = self._detect_pose(img)
            results.extend(pose_results)

        # 人形检测 - 陌生人
        if self.person_model is not None:
            person_results = self._detect_person(img)
            results.extend(person_results)

        return results

    def _detect_pose(self, img):
        """姿态检测"""
        results = []

        if self.pose_model is None:
            return results

        try:
            # 推理
            preds = self.pose_model(img, verbose=False)

            for pred in preds:
                if pred.keypoints is None:
                    continue

                keypoints = pred.keypoints.xy.cpu().numpy()
                if len(keypoints) == 0:
                    continue

                # 获取关键点
                kp = keypoints[0]  # 第一个人的关键点

                # 检查摔倒 (简单算法: 头和脚的高度差小)
                if len(kp) >= 17:
                    # 肩膀中点
                    left_shoulder = kp[5]
                    right_shoulder = kp[6]
                    shoulder_mid = (left_shoulder + right_shoulder) / 2

                    # 髋部中点
                    left_hip = kp[11]
                    right_hip = kp[12]
                    hip_mid = (left_hip + right_hip) / 2

                    # 脚
                    left_foot = kp[17] if len(kp) > 17 else kp[15]
                    right_foot = kp[18] if len(kp) > 18 else kp[16]

                    # 头部 (鼻子)
                    head = kp[0]

                    # 躯干角度
                    dx = shoulder_mid[0] - hip_mid[0]
                    dy = shoulder_mid[1] - hip_mid[1]
                    angle = abs(np.arctan2(dy, dx) * 180 / np.pi)

                    # 高度比
                    body_height = np.linalg.norm(shoulder_mid - hip_mid)
                    width = abs(left_shoulder[0] - right_shoulder[0])
                    height_ratio = body_height / (width + 1e-6)

                    # 摔倒判定
                    # 1. 躯干接近水平 (angle > 60 或 angle < 30)
                    # 2. 身体变矮
                    if (angle < 30 or angle > 60) and height_ratio < 1.5:
                        conf = float(pred.probs[0].cpu().numpy()) if hasattr(pred, 'probs') else 0.7
                        results.append({
                            "type": "fall",
                            "confidence": conf,
                            "bbox": pred.boxes.xyxy[0].cpu().numpy().tolist() if pred.boxes is not None else [],
                            "keypoints": kp.tolist(),
                        })

        except Exception as e:
            print(f"[YOLO] Pose detection error: {e}")

        return results

    def _detect_person(self, img):
        """人形检测"""
        results = []

        if self.person_model is None:
            return results

        try:
            preds = self.person_model(img, verbose=False, classes=[0])  # class 0 = person

            for pred in preds:
                if pred.boxes is None or len(pred.boxes) == 0:
                    continue

                boxes = pred.boxes.xyxy.cpu().numpy()
                confs = pred.boxes.conf.cpu().numpy()

                for box, conf in zip(boxes, confs):
                    if conf > 0.5:
                        results.append({
                            "type": "person",
                            "confidence": float(conf),
                            "bbox": box.tolist(),
                        })

        except Exception as e:
            print(f"[YOLO] Person detection error: {e}")

        return results


def simple_fall_detection(kp):
    """
    简单的摔倒检测算法 (无需模型)

    基于关键点高度变化和人体角度判断
    """
    if len(kp) < 17:
        return False, 0.0

    # 简化: 检查肩膀和髋部的相对位置
    # 正常站立时肩膀高于髋部，摔倒时接近
    left_shoulder = kp[5]
    right_shoulder = kp[6]
    shoulder_mid_y = (left_shoulder[1] + right_shoulder[1]) / 2

    left_hip = kp[11]
    right_hip = kp[12]
    hip_mid_y = (left_hip[1] + right_hip[1]) / 2

    # 高度差小于阈值说明可能摔倒
    height_diff = abs(shoulder_mid_y - hip_mid_y)
    img_height = 480  # 假设

    if height_diff < img_height * 0.1:  # 10%高度
        return True, 0.8

    return False, 0.0


if __name__ == "__main__":
    # 测试
    import numpy as np

    detector = YOLODetector()

    # 生成测试图像
    test_img = np.random.randint(0, 255, (480, 640, 3), dtype=np.uint8)

    results = detector.detect(test_img.tobytes(), 640, 480, 3)
    print(f"Detection results: {results}")
