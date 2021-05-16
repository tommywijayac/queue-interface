function refreshTime() {
    var day_INA = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu'];
    var month_INA = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'];

    var date = new Date();
    // Time
    var hour = date.getHours();
    hour = hour.length==1 ? 0 + hour : hour;
    var minute = date.getMinutes().toString();
    minute = minute.length==1 ? 0 + minute : minute;
    var second = date.getSeconds().toString();
    second = second.length==1 ? 0 + second : second;
    // Date
    var month = month_INA[date.getMonth()];
    var dateNum = date.getDate();
    dateNum = dateNum.length==1 ? 0 + dateNum : dateNum;
    var day = day_INA[date.getDay()];

    var dateText = day + ", " + dateNum + " " + month + " " + date.getFullYear();
    var timeText = hour + ":" + minute + ":" + second;
    document.getElementById("date").innerHTML = dateText;
    document.getElementById("time").innerHTML = timeText;
}
setInterval(refreshTime, 1000);