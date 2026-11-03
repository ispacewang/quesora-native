"""Check that the deliverable embeds application icons and a native manifest."""
import pathlib,struct,sys,xml.etree.ElementTree as ET
exe=pathlib.Path(sys.argv[1]);data=exe.read_bytes()
u16=lambda at:struct.unpack_from('<H',data,at)[0]
u32=lambda at:struct.unpack_from('<I',data,at)[0]
pe=u32(0x3c)
assert data[pe:pe+4]==b'PE\0\0','not a PE executable'
optional=pe+24
magic=u16(optional)
assert magic==0x20b,'expected Windows x64'
sections=optional+u16(pe+20)
entries=[]
for i in range(u16(pe+6)):
 at=sections+i*40
 entries.append((u32(at+12),u32(at+8),u32(at+20),u32(at+16)))
def offset(rva):
 for virtual,size,raw,rawsize in entries:
  if virtual<=rva<virtual+max(size,rawsize):return raw+rva-virtual
 raise AssertionError('unmapped resource RVA')
resource_rva=u32(optional+112+16)
assert resource_rva,'missing resource directory'
base=offset(resource_rva)
def directory(rel):
 at=base+rel
 result={}
 for i in range(u16(at+12)+u16(at+14)):
  ident,target=struct.unpack_from('<II',data,at+16+i*8)
  result[ident]=target
 return result
def leaf(target):
 while target&0x80000000:target=next(iter(directory(target&0x7fffffff).values()))
 at=base+target
 rva,size=struct.unpack_from('<II',data,at)
 return data[offset(rva):offset(rva)+size]
types=directory(0)
assert all(kind in types for kind in [3,14,24]),'missing icon/group/manifest resources'
group=leaf(types[14]);count=struct.unpack_from('<H',group,4)[0]
sizes=[group[6+i*14] or 256 for i in range(count)]
assert set([16,20,24,32,40,48,60,64,72,96,128,256])==set(sizes),sizes
# Byte-for-byte equality guards against accidentally restoring a second resize pass.
if len(sys.argv)>2:
 frames=pathlib.Path(sys.argv[2])
 icons=directory(types[3]&0x7fffffff)
 for i,size in enumerate(sizes):
  entry=6+i*14
  assert (group[entry+1] or 256)==size,'icon frame must be square'
  ident=struct.unpack_from('<H',group,entry+12)[0]
  payload=leaf(icons[ident])
  assert payload==(frames/('icon-'+str(size)+'.png')).read_bytes(), 'PE icon differs from prepared frame: '+str(size)
manifest=leaf(types[24])
xml=ET.fromstring(manifest)
awareness=xml.find('.//{http://schemas.microsoft.com/SMI/2016/WindowsSettings}dpiAwareness')
assert awareness is not None and 'permonitorv2' in (awareness.text or '').lower(),'missing high-DPI manifest'
print('PE verified: x64, native manifest, icon sizes '+str(sizes))
