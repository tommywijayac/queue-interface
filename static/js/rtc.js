function refreshTime() {
    var day_INA = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu'];
    var month_INA = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'];

    // var date = Date.now();
    // var dateString = day_INA[date.getDay()] + "," + date.getDate() + " " + month_INA[date.getMonth()] + " " + date.getYear();

    var dateString = new Date().toLocaleString("en-US", {
        timeZone: "Asia/Jakarta",
        hourCycle: "h24",
    });
    var formattedString = dateString.replace(", ", " - ");
    document.getElementById("time").innerHTML = formattedString;   
}
setInterval(refreshTime, 1000);