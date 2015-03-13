# sharaq

Sharaq is an image transformer. You pass a URL to Sharaq, and if the transformed 
image exists, it serves that. Otherwise, it serves the original, untransformed image
while at the same time doing the transformation in the background, so that the
next request can serve the new transformed image.

## Example

Suppose `http://sharaq.example.com` is the sharaq endpoint URL, and you want to transform
`http://images.example.com/foo/bar/baz.jpg` to `small` preset (see below for what presets
are).

You can do this by accessing the following URL

    http://sharaq.example.com/?url=http://images.example.com/foo/bar/baz.jpg&preset=small

## In Real Life / Reverse Proxy

In real life, you probably don't want to expose sharaq directly to the internet.
Using a reverse proxy minimizes the chances of a screw up, and also, you can make
URLs look a bit nicer. For example, you could accept this in your reverse proxy:

    http://sharaq.example.com/small/http://images.example.com/foo/bar/baz.jpg

and transform that to below when passing to the actual sharaq app

    http://upstream/?url=http://images.example.com/foo/bar/baz.jpg&preset=small

## Presets

Presets define a mapping from a "name" to "a set of rules to transform the image".
The rules are stolend directly from https://github.com/willnorris/imageproxy. It should
be stored in the config file, which is passed to sharaq server:

```json
{
    ...
    "Presets": {
        "square": "200x200",
        "small": "300x400",
        "big": "400x500",
    }
}
```

## Whitelist

You probably don't want to transform any image URL that was passed. For this, you should
specify the *whitelist* regular expression to filter what can be proxied.

To enable this, specify a list of URLs in the config file:

```json
{
    ...
    "Whitelist": [
        "^http(s)?://myconmpany.com/"
    ]
}
```

## Backend

Sharaq can use multiple backends. For production use you want to use the S3 backend.

### S3 Backend

The S3 backend stores all the images within a given S3 bucket. You should setup a IAM 
role to be used by the sharaq instance so access to the S3 bucket is secured. To allow 
proper access your IAM policy should look something like this:


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

Once you have the proper policies set up, add the proper credentials in the config file:

```json
{
    ...
    "AccessKey": "AMAZON IAM ACCESS KEY",
    "BackendType": "s3",
    "BucketName": "BUCKET_NAME",
    "SecretKey": "AMAZON IAM SECRET KEY",
}
```

### FS Backend

The FS backend stores all the images in a directory in the sharaq host. You probably
don't want to use this except for testing and for debugging.

```json
{
    ...
    "BackendType": "fs",
    "StorageRoot": "/path/to/storage-dir"
}
```

## URL Cache / Memcached

Sharaq stores images known to have transformed content in a cache so that it can
save on a roundtrip to check if it exists in the backend. Performance will degrade
significantly if you don't use a cache, so ... just do it :)

```
{
    ...
    "MemcachedAddr": ["mycache:11211"]
}
```
