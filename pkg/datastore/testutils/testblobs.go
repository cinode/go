/*
Copyright © 2025 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testutils

import (
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/jbenet/go-base58"
)

var TestBlobs = []struct {
	Name     *common.BlobName
	Data     []byte
	Expected []byte
}{
	{
		golang.Must(common.BlobNameFromString("KDc2ijtWc9mGxb5hP29YSBgkMLH8wCWnVimpvP3M6jdAk")),
		base58.Decode("3A836b"),
		base58.Decode("3A836b"),
	},
	{
		golang.Must(common.BlobNameFromString("BG8WaXMAckEfbCuoiHpx2oMAS4zAaPqAqrgf5Q3YNzmHx")),
		base58.Decode("AXG4Ffv"),
		base58.Decode("AXG4Ffv"),
	},
	{
		golang.Must(common.BlobNameFromString("2GLoj4Bk7SvjQngCT85gxWRu2DXCCjs9XWKsSpM85Wq3Ve")),
		base58.Decode(""),
		base58.Decode(""),
	},
	{
		golang.Must(common.BlobNameFromString("251SEdnHjwyvUqX1EZnuKruta4yHMkTDed7LGoi3nUJwhx")),
		base58.Decode("1DhLfjA9ij9QFBh7J8ysnN3uvGcsNQa7vaxKEwbYEMSEXuZbgyCtUAn5FhadZHuh7wergdpyrfuDX2TpddoWtu14HkVkFQsuHzNuPg3LAuyhQwiuKDxLtmjWkDExx651o7Gun8VYkbDVPvabYSa2Kgbei59YyUKhRztrfySngpUr17HDn38e6RT9hmmkfL8jL8FiTsqrkFgxCYKQXaBkHQBswy7rWUgP8kT65wJdAgXykW2WwyyNMKtYUiX2iLGGNDfbt4EFiJAQbPZJBtEdwnhP66hM"),
		base58.Decode("9VBV1V9DJ2uqDd99zZaCsuQp6v95vwsfuty2wGQKDPZTg4cmbRqZUgZzJkgEJWk6ps2z87M5zRQ4FisjcskpSZoSxZL4Zjpb"),
	},
	{
		golang.Must(common.BlobNameFromString("27vP1JG4VJNZvQJ4Zfhy3H5xKugurbh89B7rKTcStM9guB")),
		base58.Decode("1eE2wp1836WtQmEbjdavggJvFPU7dZbQQH5EBS2LwBL2rYjArM9mjvWFEGSfXrQCHscqdGy68exskkPXpGko2HezEAoz4UyQevHphVR5QP1JdLYYmAb4yA63bSznXz6osc8EyxvcKtLGoyfss7omAwrtGLeq1NNiYniXBiJJtuJxtKanw4GAPzn8mpoqhmZQFd36VV5MtLNFpTz5S8ke7MZSkCRKYLJutBxev9fZ5xvt2gqYWEQizWgV691juLC4FA5H82cBq2ZKwUwF4ad1JVcu822AA"),
		base58.Decode("PKfeHiNXhYXvq4nu6QKyVTgAXwiLBBJWg6LgZvpgMY82TU5WBBFMdTZQs18kD4iVpkGzH4fjupcRFZJVwJ6rouakMJF6mtvk6"),
	},
	{
		golang.Must(common.BlobNameFromString("e3T1HcdDLc73NHed2SFu5XHUQx5KDwgdAYTMmmEk2Ekqm")),
		base58.Decode("1yULPpEx3gjpKNBLCEzb2oj2xRcdGfr88CztgfYfEipBGiJCijqWBEEmVAQJ4F33AoJyYkq9Rmj6n9ChngFR7TP8jHjddQM2sKqyDi1NUAmWi7TdGCh79FXTGR12r1RNoNPfqUVv1YZjyNsCgw5cN9WetWgoj5jbdxrqkyq3UjnqM1gEfazdKCyfvWurWr3aWRy4GxQuAQDxfccpSkBxVfzchb4CyRftPt28Lc85g4qGA3oHiLDrwh1qX29gFuZqse8Nq3rLsTUT5vNiLbd1Kr"),
		base58.Decode("ULEdmCvFAc593MMZ1Yyd6etYP6ofZE8jE41hLWp7mUUs2DyfP3y9BguoyNLK5KumSLqy6vWDGG81CnMkqa8iaiL1jz"),
	},
}

var DynamicLinkPropagationData = []struct {
	Name     *common.BlobName
	Data     []byte
	Expected []byte
}{
	{
		golang.Must(common.BlobNameFromString("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8cQX3eKBQ2zdmzs9wRqESbGzCELdM9Kn7RoNmJiA7LY7hg66iPrWGfikzhJtRfFqS7eXs4ohLqbcaNXQt3i36JSxkeriTJBCEuzi86uAUqL9oJm5uqQJ7QBehXPDX7pjFZGi3QKA1JfPwsJUsZEwwfJPX2jhsZHDCgnpdRJoVaGQ6zj3u9PVoTCNqiy5m534o6Dejer4yJQWxvxeNJcRgyoCGRek1ByQGyChziW"),
		base58.Decode("PiS95EiCcNaz4dkpMGYd1hSjPzTR28Rx7nTWwb8yocFVLVJoVWoqHjE8u9FtrSfB9qUufCHHRaS95oKmFE9WTrdNRr5zkQ5xn"),
	},
	{
		golang.Must(common.BlobNameFromString("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8aKUfQY1JGDimnrVjYEythtb2CP3SrmYzHf3typ12JKUuCcrHThgZodib6AjLrhV4qZFpqX2DcRscG1oHGX13Tyny8FhGTPio6mgHYze27vPpcNFbp1jx5ETXCTHuur9UpAfag1FSakh8CnKayFegQKEav5rfbfb75Y7hnovYncSPTcerdTnFyVqjDSXNhYophu1o8Nupffv6xpeMJMmcUDWhmm5ofWDpew7x7y"),
		base58.Decode("GVqyiBNhD53H64AjZHs2hR631JBR7PcDRLSsk6mTaofpFDfZnWWGt6hHsonfW1jUFV4h87cVrQKB6kyKVkPisED1PY22bjGwR"),
	},
	{
		golang.Must(common.BlobNameFromString("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8aApx2qggEBaUKFT9T3MNwPAicht7Zjiw1Crm2ffEvxFWPupKCcGea11YG4x3NsF2u57V3Z82bhBMfXFHDPywnLnBBx3SQe688vNitGiLzyjBqH9oMCaD3oVKdyKNtE9DmNuTgCRSTnj31FiAaWWzHtqMbVqxNToVp78hhkWudpJDiqJM1Z7DNPK8RGjYDNBrtcbzxfBk4gbSL9usgAGgV7Ty3fiDAmUx8RG2vk"),
		base58.Decode("RNffUTysjj6v8JgFbNqpUL9CvnLjiDzM9fehH89p7iTFNXQtDjng1woWPnvUDvuZXSTsxw2ndUo6rfPFpZVuip29ZakLbu49n"),
	},
}
