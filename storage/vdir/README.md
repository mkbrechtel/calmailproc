## VDir Filename Specification

The vdir storage format (used in `/storage/vdir`) needs a consistent, deterministic method for converting event UIDs to filenames. Based on the vdirsyncer implementation, here are the specifications for filename handling in the vdir storage:

### Event Identity and UID Handling

In vdirsyncer, the identification of calendar items follows these principles:

1. **Primary Identifier**: The `UID` property from the calendar item is used as the primary identifier when present
   - This is extracted from the raw iCalendar data
   - If an item lacks a UID, a hash of the raw item content is used instead
   - The property `Item.ident` represents this concept - it's either the UID or the content hash

2. **Item Normalization**: 
   - vdirsyncer includes normalization for comparing items, but not for filename generation
   - Normalization removes properties like PRODID, DTSTAMP, and others
   - This is used for item comparison, not for filename generation

3. **Filename Storage Strategy**:
   - The `ident` property (UID or hash) is used to create filenames for storage
   - This creates deterministic filenames that maintain associations with the original items

### Filename Generation Process

1. **Safety Check**: First check if the UID contains only safe characters:
   - Safe character set: `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-+` 
   - Note that `@` is deliberately excluded even though allowed in the spec because some servers incorrectly encode it

2. **UID as Filename**: If the UID contains only safe characters and is not too long:
   - Use the UID directly as the base filename
   - Append the appropriate extension (`.ics` for calendar files)

3. **Fallback to Hashed Filename**: If the UID contains unsafe characters or is too long:
   - Generate a hash of the UID as the base filename
   - This ensures deterministic, reliable filenames regardless of UID content

4. **Error Handling for Long Filenames**:
   - If the filesystem rejects the filename due to length, use a hashed version instead
   - Handle OS-specific errors (ENAMETOOLONG on Unix, ENOENT on Windows)

5. **File Extensions**:
   - Primary extension: `.ics` for iCalendar files 
   - Ignore files with temporary extensions (e.g., `.tmp` or user-configurable)

### Implementation Details

1. **Current Implementation**:
   - Our implementation currently uses SHA256 hashing for all UIDs 
   - This is more conservative but less human-readable than vdirsyncer's approach
   - Our hashing approach is already consistent with vdirsyncer's raw hashing

2. **Item Hashing Strategy (from vdirsyncer)**:
   - For filename generation, vdirsyncer uses:
     - The UID directly when it's available and valid for filenames
     - A simple hash of the raw data when needed
   - Normalization is only used for other purposes (item comparison), not for filename generation

3. **Recommended Updates**:
   - Add a function to check for UID safety using a character set check
   - Use the direct UID for safe UIDs to improve human readability
   - Fall back to a hashed version only when needed
   - Handle extremely long UIDs by falling back to a hash
   - Consider adding configurability for the file extension

4. **Ignoring Temporary Files**:
   - Add support for ignoring files with specific extensions (`.tmp`, `.bak`, etc.)
   - Make the ignored extensions configurable (vdirsyncer calls this `fileignoreext`)

5. **Item.ident Concept**:
   - Implement the concept of an "ident" property similar to vdirsyncer
   - This would be either the UID if present or a hash of the normalized content
   - Use this consistently for all identification and file naming purposes

6. **Benefits of the New Approach**:
   - Better interoperability with other vdir implementations
   - More human-readable filenames when possible
   - Maintains robustness for edge cases
   - Consistent approach to filename generation
   - Future-proof for handling various Calendar UID formats

