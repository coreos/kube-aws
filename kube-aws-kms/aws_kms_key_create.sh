#!/bin/bash
#aws kms key creation - not tested!
aws kms --region=eu-west-2 create-key --description="kube-aws-assets"
{
    "KeyMetadata": {
        "CreationDate": 1526040226.017,
        "KeyState": "Enabled",
        "Arn": "arn:aws:kms:eu-west-2:717986625066:key/6db2ca6d-f86b-42c2-8ab2-4d2416d15a0d",
        "AWSAccountId": "717986625066",
        "Enabled": true,
        "KeyUsage": "ENCRYPT_DECRYPT",
        "KeyId": "6db2ca6d-f86b-42c2-8ab2-4d2416d15a0d",
        "Description": "kube-aws-assets"
    }
}