local offset64 = std.stringChars('cbf29ce484222325');  //uint64 14695981039346656037
{
  local base32_table = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567',
  local base32_inv = { [base32_table[i]]: i for i in std.range(0, 31) },


  base32(input)::
    local bytes =
      if std.type(input) == 'string' then
        std.map(function(c) std.codepoint(c), input)
      else
        input;

    local oneChar(arr, i) =
      // 5 MSB of i
      base32_table[(arr[i] & 248) >> 3];

    local twoChars(arr, i) =
      oneChar(arr, i) +
      // 3 LSB of i, 2 MSB of i+1
      base32_table[(arr[i] & 7) << 2 | (arr[i + 1] & 192) >> 6] +
      // 5 NSB of i+1
      base32_table[(arr[i + 1] & 62) >> 1];

    local threeChars(arr, i) =
      twoChars(arr, i) +
      // 1 LSB of i+1, 4 MSB of i+2
      base32_table[(arr[i + 1] & 1) << 4 | (arr[i + 2] & 240) >> 4];

    local fourChars(arr, i) =
      threeChars(arr, i) +
      // 4 LSB of i+2 + 1 MSB of i+3
      base32_table[(arr[i + 2] & 15) << 1 | (arr[i + 3] & 128) >> 7] +
      // 5 NSB of i+3
      base32_table[(arr[i + 3] & 124) >> 2];

    local aux(arr, i, r) =
      if i >= std.length(arr) then
        r
      else if i + 1 >= std.length(arr) then
        local str =
          oneChar(arr, i) +
          // 3 LSB of i
          base32_table[(arr[i] & 7) << 2] +
          '======';
        aux(arr, i + 5, r + str) tailstrict
      else if i + 2 >= std.length(arr) then
        local str =
          twoChars(arr, i) +
          // 1 LSB of i+1
          base32_table[(arr[i + 1] & 1) << 4] +
          '====';
        aux(arr, i + 5, r + str) tailstrict
      else if i + 3 >= std.length(arr) then
        local str =
          threeChars(arr, i) +
          // 4 LSB of i+2
          base32_table[(arr[i + 2] & 15) << 1] +
          '===';
        aux(arr, i + 5, r + str) tailstrict
      else if i + 4 >= std.length(arr) then
        local str =
          fourChars(arr, i) +
          // 2 LSB of i+3
          base32_table[(arr[i + 3] & 3) << 3] +
          '=';
        aux(arr, i + 5, r + str) tailstrict
      else
        local str =
          fourChars(arr, i) +
          // 2 LSB of i+3, 3 MSB of i+4
          base32_table[(arr[i + 3] & 3) << 3 | (arr[i + 4] & 224) >> 5] +
          // 5 LSB
          base32_table[(arr[i + 4] & 31)];
        aux(arr, i + 5, r + str) tailstrict;

    local sanity = std.foldl(function(r, a) r && (a < 256), bytes, true);
    if !sanity then
      error 'Can only base32 encode strings / arrays of single bytes.'
    else
      aux(bytes, 0, ''),


}
