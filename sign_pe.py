#!/usr/bin/env python3
"""
Authenticode PE signer — self-contained, no external tools needed.
Uses signify's ASN.1 definitions + cryptography + pefile to:
1. Generate a self-signed code signing certificate
2. Compute the Authenticode PE digest
3. Build the PKCS#7 SignedData structure with proper SPC types
4. Embed the signature into the PE certificate table
"""

import datetime
import hashlib
import struct
import sys
import os

from cryptography import x509
from cryptography.x509.oid import NameOID, ExtendedKeyUsageOID
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa, padding

from asn1crypto import cms, core, algos, x509 as asn1_x509
from asn1crypto.cms import (
    CertificateChoices, SignerInfo, SignerIdentifier,
    IssuerAndSerialNumber, EncapsulatedContentInfo,
)
from asn1crypto.algos import DigestAlgorithm, DigestInfo, SignedDigestAlgorithm

# Use signify's SPC definitions (proper Authenticode ASN.1 structures)
from signify.asn1 import spc
from signify.asn1.spc import (
    SpcIndirectDataContent, SpcAttributeTypeAndOptionalValue,
    SpcPeImageData, SpcLink, SpcString, SpcSpOpusInfo,
    SetOfSpcSpOpusInfo, SpcStatementType, SetOfSpcStatementType,
)

import pefile


def create_self_signed_cert():
    """Create a self-signed code signing certificate + private key."""
    key = rsa.generate_private_key(public_exponent=65537, key_size=4096)

    subject = issuer = x509.Name([
        x509.NameAttribute(NameOID.COUNTRY_NAME, "NL"),
        x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Falke AI Circuit"),
        x509.NameAttribute(NameOID.COMMON_NAME, "Falke AI Circuit Code Signing"),
    ])

    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(datetime.datetime.now(datetime.timezone.utc))
        .not_valid_after(datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=3650))
        .add_extension(
            x509.BasicConstraints(ca=False, path_length=None),
            critical=True,
        )
        .add_extension(
            x509.KeyUsage(
                digital_signature=True,
                content_commitment=False,
                key_encipherment=False,
                data_encipherment=False,
                key_agreement=False,
                key_cert_sign=False,
                crl_sign=False,
                encipher_only=False,
                decipher_only=False,
            ),
            critical=True,
        )
        .add_extension(
            x509.ExtendedKeyUsage([ExtendedKeyUsageOID.CODE_SIGNING]),
            critical=False,
        )
        .sign(key, hashes.SHA256())
    )

    return cert, key


def compute_pe_digest(pe_path, hash_algo="sha256"):
    """Compute the Authenticode digest of the PE file."""
    hasher = hashlib.new(hash_algo)

    pe = pefile.PE(pe_path, fast_load=True)
    pe.parse_data_directories(directories=[
        pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_SECURITY']
    ])

    data = pe.__data__

    opt_header_start = pe.DOS_HEADER.e_lfanew + 4 + 20  # sig + file header
    checksum_offset = opt_header_start + 64

    if pe.PE_TYPE == 0x20b:  # PE32+ (64-bit)
        sec_dir_offset = opt_header_start + 144
    else:  # PE32
        sec_dir_offset = opt_header_start + 128

    security_dir = pe.OPTIONAL_HEADER.DATA_DIRECTORY[
        pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_SECURITY']
    ]
    cert_table_offset = security_dir.VirtualAddress if security_dir.VirtualAddress > 0 else len(data)
    cert_table_size = security_dir.Size if security_dir.Size > 0 else 0

    # Hash up to checksum
    hasher.update(data[:checksum_offset])
    # Skip 4-byte checksum
    pos = checksum_offset + 4
    # Hash to security dir
    hasher.update(data[pos:sec_dir_offset])
    # Skip 8-byte security dir
    pos = sec_dir_offset + 8
    # Hash to cert table
    hasher.update(data[pos:cert_table_offset])
    # Hash after cert table if any
    if cert_table_offset + cert_table_size < len(data):
        hasher.update(data[cert_table_offset + cert_table_size:])

    pe.close()
    return hasher.digest()


def build_authenticode_signature(pe_digest, cert, private_key):
    """Build the PKCS#7 SignedData structure for Authenticode."""

    # Get cert DER and parse with asn1crypto
    cert_der = cert.public_bytes(serialization.Encoding.DER)
    asn1_cert = asn1_x509.Certificate.load(cert_der)

    # Build SpcIndirectDataContent
    spc_pe_image = SpcPeImageData({
        'flags': spc.SpcPeImageFlags({'include_resources'}),
        'file': SpcLink({'file': SpcString({'unicode': '<<<Obsolete>>>'})}),
    })

    spc_attr = SpcAttributeTypeAndOptionalValue({
        'type': '1.3.6.1.4.1.311.2.1.15',  # SPC_PE_IMAGE_DATA
        'value': spc_pe_image,
    })

    digest_info = DigestInfo({
        'digest_algorithm': DigestAlgorithm({'algorithm': '2.16.840.1.101.3.4.2.1'}),  # SHA256
        'digest': pe_digest,
    })

    spc_indirect_data = SpcIndirectDataContent({
        'data': spc_attr,
        'message_digest': digest_info,
    })

    # The content to sign is the SpcIndirectDataContent DER
    content_bytes = spc_indirect_data.dump()

    # Authenticated attributes (required by Windows Authenticode):
    # 1. Content type OID (1.3.6.1.4.1.311.2.1.4 = SPC_INDIRECT_DATA)
    # 2. Message digest of the content
    # Per RFC 2315 §9.3, the messageDigest is over the content *without* the
    # outer tag+length (i.e. .contents, not .dump()).
    content_digest = hashlib.sha256(spc_indirect_data.contents).digest()

    # Authenticated attributes for Authenticode:
    # 1. contentType (1.2.840.113549.1.9.3) = SPC_INDIRECT_DATA OID
    # 2. messageDigest (1.2.840.113549.1.9.4) = hash of the content
    from asn1crypto.cms import CMSAttribute, SetOfContentType, SetOfOctetString

    auth_attrs = cms.CMSAttributes([
        CMSAttribute({
            'type': '1.2.840.113549.1.9.3',  # contentType
            'values': cms.SetOfContentType([
                cms.ContentType('1.3.6.1.4.1.311.2.1.4'),  # SPC_INDIRECT_DATA
            ]),
        }),
        CMSAttribute({
            'type': '1.2.840.113549.1.9.4',  # messageDigest
            'values': cms.SetOfOctetString([
                core.OctetString(content_digest),
            ]),
        }),
    ])

    # The signature is over the DER encoding of the authenticated attributes
    # (with IMPLICIT [0] tagging — the SET OF is re-encoded with context tag 0)
    # asn1crypto handles this: use the .dump() of the authenticated attributes
    # but with the proper tag for signing
    # The signature in Authenticode is over the DER-encoded authenticated attributes
    # with context-specific tag 0 (not the normal SET tag)
    auth_attrs_for_signing = auth_attrs
    # We need to re-tag the SET OF as [0] IMPLICIT for signing
    # asn1crypto's CMSAttributes has _implicit_tag = (0, 'implicit') for signing
    # Actually, the signer_info authenticated_attributes field is tagged [0] IMPLICIT
    # so when we dump it for signing, we need the [0] tagged version

    # Create a re-tagged copy for signing
    from asn1crypto.core import SetOf, ParsableOctetString

    # The authenticated attributes must be DER-encoded with [0] IMPLICIT tag
    # We can get this by constructing a special tagged version
    attrs_der = auth_attrs.dump()
    # The normal dump gives SET OF tag (0x31). For signing, we need [0] (0xA0)
    # Replace the outer tag
    attrs_der_for_sig = bytes([0xA0]) + attrs_der[1:]

    signature = private_key.sign(
        attrs_der_for_sig,
        padding.PKCS1v15(),
        hashes.SHA256()
    )

    # Build signer info
    signer_info = SignerInfo({
        'version': 'v1',
        'sid': SignerIdentifier({
            'issuer_and_serial_number': IssuerAndSerialNumber({
                'issuer': asn1_cert.issuer,
                'serial_number': asn1_cert.serial_number,
            }),
        }),
        'digest_algorithm': DigestAlgorithm({'algorithm': '2.16.840.1.101.3.4.2.1'}),
        'signed_attrs': auth_attrs,
        'signature_algorithm': SignedDigestAlgorithm({'algorithm': '1.2.840.113549.1.1.11'}),  # RSA-SHA256
        'signature': signature,
    })

    # Build SignedData — for v1, asn1crypto expects ContentInfo not EncapsulatedContentInfo
    signed_data = cms.SignedData({
        'version': 'v1',
        'digest_algorithms': [DigestAlgorithm({'algorithm': '2.16.840.1.101.3.4.2.1'})],
        'encap_content_info': cms.ContentInfo({
            'content_type': '1.3.6.1.4.1.311.2.1.4',  # SPC_INDIRECT_DATA
            'content': spc_indirect_data,
        }),
        'certificates': [CertificateChoices({'certificate': asn1_cert})],
        'signer_infos': [signer_info],
    })

    # Outer PKCS#7 ContentInfo
    pkcs7 = cms.ContentInfo({
        'content_type': '1.2.840.113549.1.7.2',  # signedData
        'content': signed_data,
    })

    return pkcs7.dump()


def embed_signature(pe_path, signature_der, output_path):
    """Embed the PKCS#7 signature into the PE file's certificate table."""
    pe = pefile.PE(pe_path, fast_load=True)
    pe.parse_data_directories(directories=[
        pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_SECURITY']
    ])

    data = bytearray(pe.__data__)

    opt_header_start = pe.DOS_HEADER.e_lfanew + 4 + 20
    if pe.PE_TYPE == 0x20b:  # PE32+ (64-bit)
        sec_dir_offset = opt_header_start + 144
    else:  # PE32
        sec_dir_offset = opt_header_start + 128

    # WIN_CERTIFICATE structure
    cert_data = signature_der
    win_cert_size = 8 + len(cert_data)
    padding_needed = (8 - (win_cert_size % 8)) % 8
    win_cert_size_padded = win_cert_size + padding_needed

    win_cert = struct.pack('<IHH', win_cert_size, 0x0200, 0x0002) + cert_data + b'\x00' * padding_needed

    cert_table_offset = len(data)
    data.extend(win_cert)

    # Update security directory
    struct.pack_into('<II', data, sec_dir_offset, cert_table_offset, win_cert_size_padded)

    with open(output_path, 'wb') as f:
        f.write(data)

    pe.close()
    return cert_table_offset, win_cert_size_padded


def main():
    if len(sys.argv) < 2:
        print("Usage: sign_pe.py <input.exe> [output.exe]")
        sys.exit(1)

    input_path = sys.argv[1]
    output_path = sys.argv[2] if len(sys.argv) > 2 else input_path.replace('.exe', '-signed.exe')

    print(f"[*] Input:  {input_path}")
    print(f"[*] Output: {output_path}")

    # Step 1: Create cert
    print("[*] Generating self-signed code signing certificate...")
    cert, key = create_self_signed_cert()

    cert_pem = cert.public_bytes(serialization.Encoding.PEM)
    key_pem = key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption()
    )

    cert_path = os.path.join(os.path.dirname(os.path.abspath(output_path)), 'code-signing-cert.pem')
    key_path = os.path.join(os.path.dirname(os.path.abspath(output_path)), 'code-signing-key.pem')
    with open(cert_path, 'wb') as f:
        f.write(cert_pem)
    with open(key_path, 'wb') as f:
        f.write(key_pem)
    print(f"[*] Certificate: {cert_path}")
    print(f"[*] Private key: {key_path}")

    # Step 2: Compute digest
    print("[*] Computing Authenticode digest (SHA256)...")
    pe_digest = compute_pe_digest(input_path)
    print(f"[*] Digest: {pe_digest.hex()}")

    # Step 3: Build signature
    print("[*] Building PKCS#7 Authenticode signature...")
    signature_der = build_authenticode_signature(pe_digest, cert, key)
    print(f"[*] Signature size: {len(signature_der)} bytes")

    # Step 4: Embed
    print("[*] Embedding signature into PE certificate table...")
    offset, size = embed_signature(input_path, signature_der, output_path)
    print(f"[*] Certificate table at offset {offset}, size {size}")

    # Step 5: Verify with signify
    print("[*] Verifying signature structure...")
    from signify.authenticode import AuthenticodeSignature
    try:
        sig = AuthenticodeSignature.from_envelope(signature_der)
        print(f"[+] PASS: PKCS#7 structure valid")
        print(f"[+] Signer: {sig.signer_info.sid.issuer_and_serial_number.issuer.native.get('common_name', 'N/A')}")
    except Exception as e:
        print(f"[!] Verification error: {e}")

    print(f"\n[+] Signed binary: {output_path}")
    print(f"[+] Certificate:   {cert_path}")


if __name__ == '__main__':
    main()