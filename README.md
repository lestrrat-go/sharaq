# sharaq

<img align="right" src="./etc/sharaq.png" width="213" height="323">

`sharaq` is an image transforming web server, aimed to be run on a cloud
service provider.

Given a URL to an image and a transformation specification, sharaq will serve
a transformed version of said image. Please note that sharaq does this transformation on demand, lazily. If the requested resource has already been transformed,
the transformed version will have already been stored in the backend storage, and that content will be served. Otherwise if the transformation has not been performed yet, sharaq will first reply with the original unmodified image. Meanwhile there will be a thread that would be performing the transformation, so that when the next request comes, the transformed version is used.

# DESCRIPTION

Suppose `http://sharaq.example.com` is the sharaq endpoint URL, and you want to transform `http://images.example.com/foo/bar/baz.jpg` to `small` preset (see below for what presets are).

You can do this by accessing the following URL

    http://sharaq.example.com/?url=http://images.example.com/foo/bar/baz.jpg&preset=small

## In Real Life / Reverse Proxy

In real life, you probably don't want to expose sharaq directly to the internet. Using a reverse proxy minimizes the chances of a screw up, and also, you can make URLs look a bit nicer. For example, you could accept this in your reverse proxy:

    http://sharaq.example.com/small/http://images.example.com/foo/bar/baz.jpg

and transform that to below when passing to the actual sharaq app

    http://upstream/?url=http://images.example.com/foo/bar/baz.jpg&preset=small

# CONFIGURATION

## AWS (S3) Backend

```json
{
  "Amazon": {
    "AccessKey": "...",
    "SecretKey": "...",
    "BucketName": "..."
  }
}
```

### IAM Setup 

The S3 backend stores all the images within the specified S3 bucket. You should setup a IAM role to be used by the sharaq instance so access to the S3 bucket is secured. To allow proper access your IAM policy should look something like this:

```json
{
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket",
        "s3:GetBucketLocation",
        "s3:ListBucketMultipartUploads"
      ],
      "Resource": "arn:aws:s3:::BUCKET_NAME",
      "Condition": {}
    },
    {
      "Sid": "Stmt1420772143000",
      "Effect": "Allow",
      "Action": [
        "s3:*"
      ],
      "Resource": [
        "arn:aws:s3:::BUCKET_NAME/*"
      ]
    },
    {
      "Effect":"Allow",
      "Action": [ "s3:ListAllMyBuckets" ],
      "Resource": [ "arn:aws:s3:::*" ]
    }
  ]
}
```

## GCP (Google Storage) Backend

For GCP (Google Storage), service keys are looked under several known locations.Look at `"golang.org/x/oauth2/google".DefaultTokenSource` for details.

```json
{
  "Google": {
    "BucketName": "..."
  }
}
```
## File System Backend

The FS backend stores all the images in a directory in the sharaq host. You probably don't want to use this except for testing and for debugging.

```json
{
  "FileSystem": {
    "StorageRoot": "/path/to/storage-dir"
  }
}
```

## Presets

Presets define a mapping from a "name" to "a set of rules to transform the image".
The rules are stolend directly from https://github.com/willnorris/imageproxy. It should
be stored in the config file, which is passed to sharaq server:

```json
{
  "Presets": {
    "square": "200x200",
    "small": "300x400",
    "big": "400x500"
  }
}
```

## Whitelist

You probably don't want to transform any image URL that was passed. For this, you should
specify the *whitelist* regular expression to filter what can be proxied.

To enable this, specify a list of URLs in the config file:

```json
{
    "Whitelist": [
        "^http(s)?://myconmpany.com/"
    ]
}
```

## URL Cache

sharaq stores URL of images known to have been transformed already in a cache so that it can save on a roundtrip back to the storage backend to check if it exists. Performance will degrade significantly if you don't use a cache, so enabling the cache is highly recommended.

### Redis backend

In your configuration file, specify the following parameter to specify the servers to use

```json
{
  "URLCache": {
    "BackendType": "Redis",
    "DefaultExpires": 60
  },
  "Redis" {
    "Addr": ["mycache:6397"]
  }
}
```

### Memcache backend

In your configuration file, specify the following parameter to specify the servers to use

```json
{
  "URLCache": {
    "BackendType": "Memcached",
    "DefaultExpires": 60,
    "Memcached": {
      "Addr": ["mycache:11211"]
    }
  }
}
```

# ACKNOWLEDGEMENTS

This code was originally developed at Peatix Inc, and has since been transferred to Daisuke Maki (lestrrat)
