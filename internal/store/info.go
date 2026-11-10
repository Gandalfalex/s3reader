// Package store wraps the AWS SDK S3 calls used by the UI.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"s3reader/internal/config"
)

// LifecycleRule is one lifecycle rule rendered as human-readable lines.
type LifecycleRule struct {
	ID      string
	Status  string
	Scope   string
	Actions []string
}

type BucketInfo struct {
	Name          string
	Region        string
	Endpoint      string
	Created       time.Time
	Versioning    string
	MFADelete     string
	Encryption    []string
	Tags          [][2]string
	Lifecycle     []LifecycleRule
	LifecycleNote string // set when lifecycle could not be read or is absent
	ObjectLock    string
	Retention     string // default Object Lock retention
	PublicBlock   string
	PolicyState   string
	Owner         string
	Errors        []string
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
		bi.LifecycleNote = note("lifecycle", err)
	} else {
		for _, r := range l.Rules {
			bi.Lifecycle = append(bi.Lifecycle, lifecycleRule(r))
		}
	}

	if ol, err := c.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: &bucket}); err != nil {
		bi.ObjectLock = note("object lock", err)
	} else if cfg := ol.ObjectLockConfiguration; cfg != nil {
		bi.ObjectLock = string(cfg.ObjectLockEnabled)
		if bi.ObjectLock == "" {
			bi.ObjectLock = "none"
		}
		if cfg.Rule != nil && cfg.Rule.DefaultRetention != nil {
			dr := cfg.Rule.DefaultRetention
			switch {
			case dr.Years != nil:
				bi.Retention = fmt.Sprintf("%s, %s", dr.Mode, plural(aws.ToInt32(dr.Years), "year"))
			case dr.Days != nil:
				bi.Retention = fmt.Sprintf("%s, %s", dr.Mode, plural(aws.ToInt32(dr.Days), "day"))
			default:
				bi.Retention = string(dr.Mode)
			}
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

func lifecycleRule(r types.LifecycleRule) LifecycleRule {
	lr := LifecycleRule{ID: aws.ToString(r.ID), Status: string(r.Status), Scope: lifecycleScope(r)}
	if lr.ID == "" {
		lr.ID = "(unnamed)"
	}
	add := func(format string, a ...any) { lr.Actions = append(lr.Actions, fmt.Sprintf(format, a...)) }
	for _, t := range r.Transitions {
		switch {
		case t.Days != nil:
			add("→ %s after %s", t.StorageClass, plural(aws.ToInt32(t.Days), "day"))
		case t.Date != nil:
			add("→ %s on %s", t.StorageClass, t.Date.Format(time.DateOnly))
		}
	}
	if e := r.Expiration; e != nil {
		switch {
		case e.Days != nil:
			add("expire after %s", plural(aws.ToInt32(e.Days), "day"))
		case e.Date != nil:
			add("expire on %s", e.Date.Format(time.DateOnly))
		}
		if aws.ToBool(e.ExpiredObjectDeleteMarker) {
			add("remove expired delete markers")
		}
	}
	for _, t := range r.NoncurrentVersionTransitions {
		add("noncurrent → %s after %s", t.StorageClass, plural(aws.ToInt32(t.NoncurrentDays), "day"))
	}
	if n := r.NoncurrentVersionExpiration; n != nil {
		s := "expire noncurrent after " + plural(aws.ToInt32(n.NoncurrentDays), "day")
		if n.NewerNoncurrentVersions != nil {
			s += fmt.Sprintf(" (keep %d newest)", aws.ToInt32(n.NewerNoncurrentVersions))
		}
		add("%s", s)
	}
	if a := r.AbortIncompleteMultipartUpload; a != nil {
		add("abort incomplete uploads after %s", plural(aws.ToInt32(a.DaysAfterInitiation), "day"))
	}
	return lr
}

func lifecycleScope(r types.LifecycleRule) string {
	var parts []string
	addPrefix := func(p *string) {
		if p != nil && *p != "" {
			parts = append(parts, "prefix "+*p)
		}
	}
	addTag := func(t types.Tag) { parts = append(parts, "tag "+aws.ToString(t.Key)+"="+aws.ToString(t.Value)) }
	addSize := func(gt, lt *int64) {
		if gt != nil {
			parts = append(parts, fmt.Sprintf("size > %d B", *gt))
		}
		if lt != nil {
			parts = append(parts, fmt.Sprintf("size < %d B", *lt))
		}
	}
	addPrefix(r.Prefix)
	if f := r.Filter; f != nil {
		addPrefix(f.Prefix)
		if f.Tag != nil {
			addTag(*f.Tag)
		}
		addSize(f.ObjectSizeGreaterThan, f.ObjectSizeLessThan)
		if a := f.And; a != nil {
			addPrefix(a.Prefix)
			for _, t := range a.Tags {
				addTag(t)
			}
			addSize(a.ObjectSizeGreaterThan, a.ObjectSizeLessThan)
		}
	}
	if len(parts) == 0 {
		return "whole bucket"
	}
	return strings.Join(parts, ", ")
}

func plural(n int32, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// errorNotes maps S3 error codes from bucket info calls to short UI notes.
var errorNotes = map[string]string{
	"ServerSideEncryptionConfigurationNotFoundError": "none",
	"NoSuchTagSet":                         "none",
	"NoSuchLifecycleConfiguration":         "none",
	"NoSuchPublicAccessBlockConfiguration": "none",
	"ObjectLockConfigurationNotFoundError": "none",
	"NoSuchBucketPolicy":                   "none",
	"AccessDenied":                         "access denied",
	"AccessDeniedException":                "access denied",
	"NotImplemented":                       "not supported",
	"XNotImplemented":                      "not supported",
}
