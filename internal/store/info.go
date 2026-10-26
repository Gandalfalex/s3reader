// Package store wraps the AWS SDK S3 calls used by the UI.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"s3reader/internal/config"
)

type BucketInfo struct {
	Name        string
	Region      string
	Endpoint    string
	Created     time.Time
	Versioning  string
	MFADelete   string
	Encryption  []string
	Tags        [][2]string
	Lifecycle   []string
	PublicBlock string
	PolicyState string
	Owner       string
	Errors      []string
}

func FetchBucketInfo(ctx context.Context, c *s3.Client, p config.Profile, bucket, region string, created time.Time) BucketInfo {
	bi := BucketInfo{Name: bucket, Region: region, Endpoint: p.Endpoint, Created: created}
	if bi.Endpoint == "" {
		bi.Endpoint = "AWS S3"
	}
	note := func(what string, err error) string {
		slog.Debug("bucket info call failed", "bucket", bucket, "call", what, "err", err)
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if s, ok := errorNotes[ae.ErrorCode()]; ok {
				return s
			}
			return ae.ErrorCode()
		}
		return err.Error()
	}

	if v, err := c.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: &bucket}); err != nil {
		bi.Versioning = note("versioning", err)
	} else {
		bi.Versioning = string(v.Status)
		if bi.Versioning == "" {
			bi.Versioning = "Never enabled"
		}
		bi.MFADelete = string(v.MFADelete)
	}

	if e, err := c.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: &bucket}); err != nil {
		bi.Encryption = []string{note("encryption", err)}
	} else if e.ServerSideEncryptionConfiguration != nil {
		for _, r := range e.ServerSideEncryptionConfiguration.Rules {
			if d := r.ApplyServerSideEncryptionByDefault; d != nil {
				s := string(d.SSEAlgorithm)
				if d.KMSMasterKeyID != nil {
					s += " (" + aws.ToString(d.KMSMasterKeyID) + ")"
				}
				if aws.ToBool(r.BucketKeyEnabled) {
					s += " +bucket key"
				}
				bi.Encryption = append(bi.Encryption, s)
			}
		}
	}

	if t, err := c.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: &bucket}); err == nil {
		for _, tag := range t.TagSet {
			bi.Tags = append(bi.Tags, [2]string{aws.ToString(tag.Key), aws.ToString(tag.Value)})
		}
	} else {
		note("tagging", err)
	}

	if l, err := c.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: &bucket}); err != nil {
		bi.Lifecycle = []string{note("lifecycle", err)}
	} else {
		for _, r := range l.Rules {
			bi.Lifecycle = append(bi.Lifecycle, fmt.Sprintf("%s [%s]", aws.ToString(r.ID), r.Status))
		}
	}

	if pab, err := c.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: &bucket}); err != nil {
		bi.PublicBlock = note("public access", err)
	} else if cfg := pab.PublicAccessBlockConfiguration; cfg != nil {
		all := aws.ToBool(cfg.BlockPublicAcls) && aws.ToBool(cfg.IgnorePublicAcls) &&
			aws.ToBool(cfg.BlockPublicPolicy) && aws.ToBool(cfg.RestrictPublicBuckets)
		if all {
			bi.PublicBlock = "all public access blocked"
		} else {
			bi.PublicBlock = fmt.Sprintf("blockAcls=%v ignoreAcls=%v blockPolicy=%v restrict=%v",
				aws.ToBool(cfg.BlockPublicAcls), aws.ToBool(cfg.IgnorePublicAcls),
				aws.ToBool(cfg.BlockPublicPolicy), aws.ToBool(cfg.RestrictPublicBuckets))
		}
	}

	if ps, err := c.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: &bucket}); err != nil {
		bi.PolicyState = note("policy", err)
	} else if ps.PolicyStatus != nil {
		if aws.ToBool(ps.PolicyStatus.IsPublic) {
			bi.PolicyState = "PUBLIC"
		} else {
			bi.PolicyState = "not public"
		}
	}

	if acl, err := c.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: &bucket}); err != nil {
		note("acl", err)
	} else if acl.Owner != nil {
		bi.Owner = aws.ToString(acl.Owner.DisplayName)
		if bi.Owner == "" {
			bi.Owner = aws.ToString(acl.Owner.ID)
		}
	}
	return bi
}

// errorNotes maps S3 error codes from bucket info calls to short UI notes.
var errorNotes = map[string]string{
	"ServerSideEncryptionConfigurationNotFoundError": "none",
	"NoSuchTagSet":                         "none",
	"NoSuchLifecycleConfiguration":         "none",
	"NoSuchPublicAccessBlockConfiguration": "none",
	"NoSuchBucketPolicy":                   "none",
	"AccessDenied":                         "access denied",
	"AccessDeniedException":                "access denied",
	"NotImplemented":                       "not supported",
	"XNotImplemented":                      "not supported",
}
