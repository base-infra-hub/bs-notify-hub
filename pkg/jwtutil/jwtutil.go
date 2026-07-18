package jwtutil

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ParseRSAPublicKey 解析 RSA 公钥
// 支持两种格式：
//  1. 完整 PEM（含 -----BEGIN PUBLIC KEY----- 头尾）
//  2. 裸 Base64 内容（不含头尾，直接是 PKIX DER 的 base64）
func ParseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("公钥内容为空")
	}

	var derBytes []byte

	if strings.HasPrefix(raw, "-----") {
		// 完整 PEM 格式
		block, _ := pem.Decode([]byte(raw))
		if block == nil {
			return nil, errors.New("PEM 解码失败：未找到有效的 PEM 块")
		}
		derBytes = block.Bytes
	} else {
		// 裸 Base64 内容，清理空白后解码
		cleaned := strings.ReplaceAll(raw, "\n", "")
		cleaned = strings.ReplaceAll(cleaned, "\r", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		var err error
		derBytes, err = base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, fmt.Errorf("Base64 解码失败: %w", err)
		}
	}

	// 优先尝试 PKIX 格式（标准公钥）
	pub, err := x509.ParsePKIXPublicKey(derBytes)
	if err != nil {
		// 降级尝试 PKCS1 格式
		pkcs1Key, err2 := x509.ParsePKCS1PublicKey(derBytes)
		if err2 != nil {
			return nil, fmt.Errorf("公钥解析失败（PKIX: %v; PKCS1: %v）", err, err2)
		}
		return pkcs1Key, nil
	}

	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥类型错误：不是 RSA 公钥")
	}
	return rsaKey, nil
}

// ValidateToken 验证 JWT token，返回 claims
//
// 规则：
//   - 使用 RSA 公钥校验签名（仅支持 RS256）
//   - 若 exp 字段缺失或为 null，视为永不过期（不做过期校验）
//   - 若 exp 字段存在且有值，正常校验是否已过期
//   - 若 expectedTag 非空，校验 claims 中的 tag 字段是否与之一致
func ValidateToken(tokenStr string, pubKey *rsa.PublicKey, expectedTag string) (jwt.MapClaims, error) {
	// 去掉可能携带的 Bearer 前缀
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)

	token, err := jwt.Parse(tokenStr,
		func(t *jwt.Token) (interface{}, error) {
			if t.Method.Alg() != "RS256" {
				return nil, fmt.Errorf("不支持的签名算法: %v（仅支持 RS256）", t.Header["alg"])
			}
			return pubKey, nil
		},
		// 不强制要求 exp 字段存在（exp 缺失或 null 均视为永不过期）
		jwt.WithoutClaimsValidation(),
	)
	if err != nil {
		return nil, fmt.Errorf("JWT 解析失败: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("JWT 签名无效")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("无效的 JWT claims 格式")
	}

	// 手动校验 exp（仅在 exp 存在且非 null 时才校验）
	if err := validateExpIfPresent(claims); err != nil {
		return nil, err
	}

	// 校验 tag（如果调用方传了期望值）
	if expectedTag != "" {
		if err := validateTag(claims, expectedTag); err != nil {
			return nil, err
		}
	}

	return claims, nil
}

// validateExpIfPresent 仅在 exp 存在且有值时校验是否过期
// exp 缺失 或 exp == null 均视为永不过期
func validateExpIfPresent(claims jwt.MapClaims) error {
	expVal, exists := claims["exp"]
	if !exists || expVal == nil {
		// exp 不存在或为 null，永不过期
		return nil
	}

	// exp 存在，走标准过期校验
	// golang-jwt MapClaims 支持直接调用 GetExpirationTime
	expTime, err := claims.GetExpirationTime()
	if err != nil {
		return fmt.Errorf("exp 字段格式错误: %w", err)
	}
	if expTime == nil {
		// 解析后仍为 nil，视为永不过期
		return nil
	}

	// 使用 jwt 内置的时间比较（考虑 leeway）
	validator := jwt.NewValidator(jwt.WithExpirationRequired())
	if err := validator.Validate(claims); err != nil {
		return fmt.Errorf("JWT 已过期: %w", err)
	}
	return nil
}

// validateTag 校验 JWT claims 中的 tag 字段
func validateTag(claims jwt.MapClaims, expectedTag string) error {
	tagVal, exists := claims["tag"]
	if !exists || tagVal == nil {
		return fmt.Errorf("JWT 缺少 tag 字段（期望: %s）", expectedTag)
	}
	tagStr, ok := tagVal.(string)
	if !ok {
		return fmt.Errorf("JWT tag 字段类型错误（期望 string）")
	}
	if tagStr != expectedTag {
		return fmt.Errorf("JWT tag 不匹配（期望: %s，实际: %s）", expectedTag, tagStr)
	}
	return nil
}
